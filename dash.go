package dashdog

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	css "github.com/andybalholm/cascadia"
	"github.com/go-resty/resty/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	maxDownloadedSize    = 10000
	maxRefsSize          = 50000
	maxResourcesPerLevel = 1000
	maxFileSize          = 10 * 1024 * 1024 // 10MB
	httpTimeout          = 30 * time.Second
)

type fetchItem struct {
	u            *url.URL
	level        int
	needPopulate bool
	suffix       string

	resp *resty.Response
}

func newFetchItem(u *url.URL, level int, needPopulate bool, httpClient *resty.Client) (*fetchItem, error) {
	i := &fetchItem{
		u:            u,
		level:        level,
		needPopulate: needPopulate,
		suffix:       "",
	}

	resp, err := httpClient.R().Get(u.String())
	if err != nil {
		return nil, errors.Errorf("head of %s", u.String())
	}

	contentType := resp.Header().Get("Content-Type")
	i.adjustSuffix(contentType)

	if needPopulate {
		// only html need to populate
		i.needPopulate = strings.Contains(contentType, "text/html")
	}
	i.resp = resp

	return i, nil
}

func (i *fetchItem) adjustSuffix(contentType string) {
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "image/svg+xml") && !strings.HasSuffix(i.u.Path, ".svg") {
		i.suffix = ".svg"
	} else if strings.Contains(contentType, "text/html") && !strings.HasSuffix(i.u.Path, ".html") {
		i.suffix = ".html"
	}

}

func (i fetchItem) localPath() string {
	if strings.HasSuffix(i.u.Path, i.suffix) {
		return i.u.Host + i.u.Path
	}
	return i.u.Host + i.u.Path + i.suffix
}

func (i fetchItem) localURL(prefix string) string {
	cu := *i.u
	if !strings.HasSuffix(cu.Path, i.suffix) {
		cu.Path += i.suffix
	}
	str := cu.String()
	prefixToRemove := cu.Scheme + "://"
	if strings.HasPrefix(str, prefixToRemove) {
		str = str[len(prefixToRemove):]
	}
	return prefix + str
}

func (item fetchItem) String() string {
	return fmt.Sprintf("url:%s localPath:%s level:%d needPopulate:%v", item.u.String(), item.localPath(), item.level, item.needPopulate)
}

type Dash struct {
	tree          *docTree
	db            *sql.DB
	httpClient    *resty.Client
	config        Config
	indexFilePath string
	downloaded    map[string]bool

	fetchQueue             []*fetchItem
	fetchPathRegex         *regexp.Regexp
	subPathBundleNameRegex *regexp.Regexp

	refs []*Reference
}

type Reference struct {
	name      string
	etype     string
	bundle    string
	localPath string
	anchor    string
}

func (r Reference) href() string {
	base := fmt.Sprintf(`<dash_entry_name=%s><dash_entry_originalName=%s.%s><dash_entry_menuDescription=%s>%s`,
		r.name, r.bundle, r.name, r.bundle, r.localPath)
	if r.anchor != "" {
		base += "#" + r.anchor
	}
	return base
}

func (r Reference) String() string {
	return fmt.Sprintf("name:%s type:%s href:%s", r.name, r.etype, r.href())
}

// sanitizeName removes or replaces problematic characters from docset names
func sanitizeName(name string) string {
	// Remove or replace problematic characters
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "*", "-")
	name = strings.ReplaceAll(name, "?", "-")
	name = strings.ReplaceAll(name, "\"", "-")
	name = strings.ReplaceAll(name, "<", "-")
	name = strings.ReplaceAll(name, ">", "-")
	name = strings.ReplaceAll(name, "|", "-")

	// Remove leading/trailing dots and spaces
	name = strings.Trim(name, ". ")

	// Limit length
	if len(name) > 100 {
		name = name[:100]
	}

	return name
}

func NewDash(config Config) (*Dash, error) {
	if config.Depth == 0 {
		config.Depth = 1
	}
	config.Name = sanitizeName(config.Name)

	d := &Dash{
		httpClient: resty.New().SetTimeout(httpTimeout).SetHeader("User-Agent", "dashdog/1.0"),
		tree:       newDocTree(config.Path, config.Name),
		config:     config,
		downloaded: map[string]bool{},
	}

	var err error
	if config.SubPathRegex != "" {
		d.fetchPathRegex, err = regexp.Compile(config.SubPathRegex)
		if err != nil {
			return nil, errors.Wrapf(err, "regexp.Compile SubPathRegex %s", config.SubPathRegex)
		}
	}
	if config.SubPathBundleName.Pattern != "" {
		d.subPathBundleNameRegex, err = regexp.Compile(config.SubPathBundleName.Pattern)
		if err != nil {
			return nil, errors.Wrapf(err, "regexp.Compile SubPathBundleName.Pattern %s", config.SubPathBundleName.Pattern)
		}
	}

	return d, nil
}

func (d *Dash) Build() error {
	slog.Info("build", slog.String("name", d.config.Name))
	slog.Debug("build", slog.Any("config", d.config))

	// remove old data if exist
	if err := d.tree.Rm(); err != nil {
		return errors.Wrapf(err, "rm")
	}
	slog.Debug("remove docpath", slog.String("path", d.tree.Documents()))

	// Create the docset folder
	if err := d.tree.Mkdir(); err != nil {
		return errors.Wrapf(err, "mkdir")
	}
	slog.Debug("mkdir", slog.String("path", d.tree.Documents()))

	// create sqlite index
	if err := d.createDB(); err != nil {
		return errors.Wrapf(err, "createDB")
	}
	slog.Debug("open db", slog.String("path", d.tree.DB()))
	defer func() {
		if d.db != nil {
			d.db.Close()
		}
	}()

	u, err := url.Parse(d.config.URL)
	if err != nil {
		return errors.Wrapf(err, "Parse %s", d.config.URL)
	}
	slog.Debug("parse url", slog.String("url", d.config.URL))

	item, err := newFetchItem(u, 0, true, d.httpClient)
	if err != nil {
		return errors.Wrapf(err, "newFetchItem %+v", u)
	}

	d.indexFilePath = item.localPath()

	// create the info.plist
	if err := d.infoPlist(); err != nil {
		return errors.Wrap(err, "infoPlist")
	}
	slog.Debug("create info.plist", slog.String("path", d.tree.InfoPlist()))

	slog.Debug("popItem", slog.String("item", item.String()))
	if _, err := d.populateData(item); err != nil {
		return errors.Wrapf(err, "populateData")
	}

	// sort.Slice(d.refs, func(i, j int) bool {
	// 	if d.refs[i].bundle != d.refs[j].bundle {
	// 		return d.refs[i].bundle < d.refs[j].bundle
	// 	}
	// 	return d.refs[i].name < d.refs[j].name
	// })

	if err := d.insertDB(); err != nil {
		return errors.Wrapf(err, "insertDB")
	}
	slog.Debug("insertDB", slog.String("item", item.String()), slog.Int("len(d.refs)", len(d.refs)))

	return nil
}

func (d Dash) saveFile(localPath string, body []byte) error {
	// Clean the path and check for traversal
	cleanPath := filepath.Clean(localPath)
	if strings.HasPrefix(cleanPath, "../") || strings.Contains(cleanPath, "/../") {
		return errors.Errorf("path traversal detected: %s", localPath)
	}

	dirname := filepath.Dir(cleanPath)
	absPath := filepath.Join(d.tree.Documents(), dirname)

	// Verify absPath is still within Documents
	if !strings.HasPrefix(absPath, d.tree.Documents()) {
		return errors.Errorf("path escapes Documents directory: %s", localPath)
	}

	err := os.MkdirAll(absPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "MkdirAll %s", absPath)
	}
	slog.Debug("mkdir", slog.String("path", absPath))

	absPath = filepath.Join(d.tree.Documents(), cleanPath)

	// Verify final path is still within Documents
	if !strings.HasPrefix(absPath, d.tree.Documents()) {
		return errors.Errorf("path escapes Documents directory: %s", localPath)
	}

	info, err := os.Stat(absPath)
	if err == nil {
		if info.IsDir() {
			return errors.Errorf("path exists as directory: %s", absPath)
		}
		return nil // File exists, skip
	}
	if !os.IsNotExist(err) {
		return errors.Wrapf(err, "Stat file %s", localPath)
	}

	if len(body) > maxFileSize {
		return errors.Errorf("file size %d exceeds maximum %d", len(body), maxFileSize)
	}

	err = os.WriteFile(absPath, body, 0644)
	if err != nil {
		return errors.Wrapf(err, "WriteFile %s", absPath)
	}
	slog.Debug("write file", slog.String("path", absPath))

	return nil
}

func (d Dash) infoPlist() error {
	m := &InfoPlistModel{
		CFBundleIdentifier:   d.config.Plist.CFBundleIdentifier,
		CFBundleName:         d.config.Plist.CFBundleName,
		DocSetPlatformFamily: d.config.Plist.DocSetPlatformFamily,
		DashIndexFilePath:    d.indexFilePath,
		DashDocSetPlayURL:    d.config.Plist.DashDocSetPlayURL,
		IsJavaScriptEnabled:  d.config.Plist.IsJavaScriptEnabled,
		// DashDocSetFallbackURL: d.config.URL,
	}

	t, err := template.New("info_plist").Parse(plistTpl)
	if err != nil {
		return errors.Wrap(err, "Parse plistTpl")
	}

	file, err := os.Create(d.tree.InfoPlist())
	if err != nil {
		return errors.Wrapf(err, "Create %s", d.tree.InfoPlist())
	}
	defer file.Close()

	err = t.Execute(file, m)
	if err != nil {
		return errors.Wrapf(err, "Execute m:%+v", m)
	}
	return nil
}

func (d *Dash) createDB() error {
	dbname := d.tree.DB()

	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return errors.Wrapf(err, "Open db, %s", dbname)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS searchIndex(id INTEGER PRIMARY KEY, name TEXT, type TEXT, path TEXT)`); err != nil {
		db.Close()
		return errors.Wrapf(err, "create table")
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS anchor ON searchIndex (name, type, path)`); err != nil {
		db.Close()
		return errors.Wrapf(err, "create unique index")
	}
	if _, err := db.Exec(`DELETE FROM searchIndex`); err != nil {
		db.Close()
		return errors.Wrapf(err, "delete rows")
	}

	d.db = db
	return nil
}

func (d *Dash) populateData(item *fetchItem) (*fetchItem, error) {
	checkPath := item.u.Host + item.u.Path
	if d.downloaded[checkPath] {
		slog.Debug("downloaded", slog.String("path", checkPath))
		return item, nil
	}

	// Check memory limits
	if len(d.downloaded) >= maxDownloadedSize {
		slog.Warn("downloaded map too large", slog.Int("size", len(d.downloaded)))
	}
	d.downloaded[checkPath] = true

	urlStr := item.u.String()
	resp := item.resp

	if resp.StatusCode() == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode() != http.StatusOK {
		bodyStr := string(resp.Body())
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return nil, errors.Errorf("%s status %s, body: %s", urlStr, resp.Status(), bodyStr)
	}

	// Check content-type for non-HTML
	contentType := resp.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		slog.Debug("non-HTML content", slog.String("url", urlStr), slog.String("content-type", contentType))
	}

	if !item.needPopulate {
		err := d.saveFile(item.localPath(), resp.Body())
		slog.Debug("download resource", slog.String("localPath", item.localPath()), slog.String("url", item.u.String()))
		if err != nil {
			return nil, errors.Wrapf(err, "saveFile")
		}
		return item, nil
	}

	slog.Debug("populateData url", slog.String("url", urlStr))

	u := item.u

	r := bytes.NewReader(resp.Body())
	doc, err := html.Parse(r)
	if err != nil {
		return nil, errors.Wrapf(err, "Parse html of %s", urlStr)
	}

	// remove nodes before fetch resource
	// and then we can not download the resource we don't need
	d.removeNode(doc)
	slog.Debug("removeNode", slog.String("item", item.String()))
	d.setAttr(doc)
	slog.Debug("setAttr", slog.String("item", item.String()))

	err = d.fetchResource(u, doc, item.level)
	if err != nil {
		return nil, errors.Wrapf(err, "fetchResource %s", urlStr)
	}
	slog.Debug("fetchResource", slog.String("item", item.String()))

	subRefs := d.insertAnchor(u, item.localPath(), doc)
	slog.Debug("insertAnchor", slog.String("item", item.String()))

	d.insertOnlineRedirection(doc, urlStr)
	slog.Debug("insertOnlineRedirection", slog.String("url", urlStr))

	d.insertLink(doc)
	slog.Debug("insertLink", slog.String("item", item.String()))

	err = d.writeHTML(item.localPath(), doc)
	if err != nil {
		return nil, errors.Wrap(err, "writeHTML")
	}
	slog.Debug("write html", slog.String("path", item.localPath()))

	bundleName := d.bundleNameOfPath(item.u.Path)
	pkgRef := &Reference{
		name:      bundleName,
		etype:     "Package",
		bundle:    bundleName,
		localPath: item.localPath(),
		anchor:    "",
	}
	slog.Debug("insert package", slog.String("name", pkgRef.name), slog.String("type", pkgRef.etype), slog.String("href", pkgRef.href()))

	// Check refs size limit
	if len(d.refs) >= maxRefsSize {
		slog.Warn("refs slice too large", slog.Int("size", len(d.refs)))
	} else {
		d.refs = append(d.refs, pkgRef)
	}

	// Add subRefs with limit check
	for _, ref := range subRefs {
		if len(d.refs) >= maxRefsSize {
			slog.Warn("refs slice too large, stopping additions", slog.Int("size", len(d.refs)))
			break
		}
		d.refs = append(d.refs, ref)
	}

	return item, nil
}

func (d Dash) bundleNameOfPath(path string) string {
	if path == "" {
		return d.config.Plist.CFBundleName
	}
	if d.config.SubPathBundleName.Replace == "" {
		return d.config.Plist.CFBundleName
	}
	if d.subPathBundleNameRegex == nil {
		return d.config.Plist.CFBundleName
	}
	if !d.subPathBundleNameRegex.MatchString(path) {
		return d.config.Plist.CFBundleName
	}

	return d.subPathBundleNameRegex.ReplaceAllString(path, d.config.SubPathBundleName.Replace)
}

func (d Dash) insertOnlineRedirection(doc *html.Node, urlStr string) {
	node := &html.Node{
		Type: html.CommentNode,
		Data: fmt.Sprintf("Online page at %s", urlStr),
	}

	selector := css.MustCompile("html")
	page := selector.MatchFirst(doc)
	if page != nil {
		page.InsertBefore(node, page.FirstChild)
		slog.Debug("insert online redirection", slog.Any("node", node))
	}
}

func (d Dash) removeNode(doc *html.Node) {
	for _, sel := range d.config.Page.RemoveNodeSelector {
		nodeSelector := css.MustCompile(sel)
		nodes := nodeSelector.MatchAll(doc)
		for _, node := range nodes {
			if node.Parent == nil {
				slog.Debug("node has no parent", slog.String("selector", sel))
				continue
			}
			node.Parent.RemoveChild(node)
			slog.Debug("remove node", slog.String("selector", sel), slog.Any("node", anyJson(node)))
		}
	}
}

func (d Dash) setAttr(doc *html.Node) {
	for _, sattr := range d.config.Page.SetAttrs {
		if sattr.Attr.Key == "" {
			slog.Debug("empty attribute key", slog.Any("attr", sattr))
			continue
		}
		sel := sattr.Selector
		nodeSelector := css.MustCompile(sel)
		nodes := nodeSelector.MatchAll(doc)
		for _, node := range nodes {
			found := false
			for i, attr := range node.Attr {
				if attr.Key == sattr.Attr.Key {
					// the node has the attr to set value
					node.Attr[i].Val = sattr.Attr.Value
					found = true
					slog.Debug("set attr", slog.Any("sattr", anyJson(sattr)), slog.Any("node", anyJson(node)))
					break
				}
			}
			if !found {
				slog.Debug("add attr", slog.Any("sattr", anyJson(sattr)), slog.Any("node", anyJson(node)))
				node.Attr = append(node.Attr, html.Attribute{
					Key: sattr.Attr.Key,
					Val: sattr.Attr.Value,
				})
			}
		}
	}
}

func (d *Dash) fetchResource(ourl *url.URL, doc *html.Node, level int) error {
	slog.Debug("fetchResource", slog.String("url", ourl.String()), slog.Int("level", level))
	prefix := pathRelativeToRoot(ourl.Path)

	resourceSelector := css.MustCompile("*[href],*[src]")
	nodes := resourceSelector.MatchAll(doc)

	// Limit resources per level
	if len(nodes) > maxResourcesPerLevel {
		slog.Warn("too many resources", slog.Int("count", len(nodes)), slog.Int("limit", maxResourcesPerLevel))
		nodes = nodes[:maxResourcesPerLevel]
	}

	for _, node := range nodes {
		for i, attr := range node.Attr {
			if attr.Key != "href" && attr.Key != "src" {
				continue
			}

			u, err := url.Parse(attr.Val)
			if err != nil {
				return errors.Wrapf(err, "Parse %s", attr.Val)
			}

			// Validate scheme - whitelist safe schemes
			switch u.Scheme {
			case "http", "https", "":
				// OK
			default:
				slog.Debug("skipping unsupported scheme", slog.String("scheme", u.Scheme), slog.String("url", attr.Val))
				continue
			}

			if u.Scheme == "" {
				u.Scheme = ourl.Scheme
			}
			if u.Host == "" {
				u.Host = ourl.Host
			}
			if u.Path == "" {
				u.Path = ourl.Path
			}

			// not a atom.A => set a relative url, push to queue
			// is atom.A
			//   from a difference site => set the whole url
			//   from the same site, same url => set the relative url
			//   from the same site, not a sub page => set the whole url
			//   from the same site, is a sub page => set the relative url, push to queue

			if node.DataAtom != atom.A {
				item, err := newFetchItem(u, level, false, d.httpClient)
				if err != nil {
					return errors.Wrapf(err, "newFetchItem")
				}

				if item.resp.StatusCode() == http.StatusNotFound {
					slog.Error("populateData failed", slog.Any("item", item), slog.String("err", fmt.Sprintf("%+v", err)))
					if node.Parent != nil {
						node.Parent.RemoveChild(node)
					}
					break
				} else if item.resp.StatusCode() != http.StatusOK {
					return errors.Wrapf(err, "newFetchItem")
				}

				slog.Debug("process item", slog.String("item", item.String()), slog.Any("node", node), slog.Any("attr", attr))

				item, err = d.populateData(item)
				if err != nil {
					return errors.Wrapf(err, "populateData")
				} else {
					node.Attr[i].Val = item.localURL(prefix)
				}

				continue
			}

			if ourl.Host != u.Host {
				node.Attr[i].Val = u.String()
				continue
			}

			if ourl.Path == u.Path {
				node.Attr[i].Val = relativeURL(prefix, u, ".html")
				continue
			}

			if !strings.HasPrefix(u.Path, ourl.Path) {
				node.Attr[i].Val = u.String()
				continue
			}

			if level+1 <= d.config.Depth-1 && d.pathMatchRegex(u.Path) {
				item, err := newFetchItem(u, level+1, true, d.httpClient)
				if err != nil {
					return errors.Wrapf(err, "newFetchItem")
				}
				slog.Debug("process item", slog.String("item", item.String()))
				item, err = d.populateData(item)

				if err != nil {
					return errors.Wrapf(err, "populateData")
				} else {
					node.Attr[i].Val = item.localURL(prefix)
					slog.Debug("populateData data succ", slog.Any("item", item), slog.String("attr.val", node.Attr[i].Val))
				}
			} else {
				node.Attr[i].Val = u.String()
			}

		}
	}
	return nil
}

func (d Dash) pathMatchRegex(path string) bool {
	if d.fetchPathRegex == nil {
		return true
	}
	match := d.fetchPathRegex.MatchString(path)
	slog.Debug("MatchString", slog.String("path", path), slog.String("pattern", d.config.SubPathRegex), slog.Bool("match", match))
	return match
}

func (d Dash) insertLink(doc *html.Node) {
	anchorSelector := css.MustCompile(".dashAnchor")
	anchors := anchorSelector.MatchAll(doc)

	links := make([]*html.Node, 0, len(anchors))
	for _, anchor := range anchors {
		link := newLinkFromNode(anchor)
		links = append(links, link)
	}

	headSelector := css.MustCompile("head")
	head := headSelector.MatchFirst(doc)
	if head != nil {
		for _, link := range links {
			head.InsertBefore(link, head.LastChild)
			slog.Debug("insert link", slog.String("href", attr(link, "href")))
		}
	}

}

func (d Dash) insertAnchor(u *url.URL, localPath string, doc *html.Node) []*Reference {
	refs := make([]*Reference, 0)

	for _, sel := range d.config.Index.IndexRows {
		nodeSelector := css.MustCompile(sel.Selector)
		nodes := nodeSelector.MatchAll(doc)
		for _, node := range nodes {
			name := ""
			switch sel.Name.Type {
			case IndexNameTypeText:
				name = text(node)
			case IndexNameTypeAttr:
				name = attr(node, sel.Name.Value)
			case IndexNameTypeConstant:
				name = sel.Name.Value
			}

			// Sanitize name for database
			name = strings.TrimSpace(name)
			name = strings.ReplaceAll(name, "\n", " ")
			name = strings.ReplaceAll(name, "\r", "")
			name = strings.ReplaceAll(name, "\t", " ")
			// Remove control characters
			name = strings.Map(func(r rune) rune {
				if r < 32 || r == 127 {
					return -1
				}
				return r
			}, name)

			if name == "" {
				slog.Debug("name is empty", slog.Any("node", anyJson(node)))
				continue
			}

			a := newA(name, sel.Type, sel.Level)

			if !sel.AnchorOnly {
				anchor := attr(a, "name")
				bundle := d.bundleNameOfPath(u.Path)
				ref := &Reference{
					name:      name,
					etype:     sel.Type,
					bundle:    bundle,
					localPath: localPath,
					anchor:    anchor,
				}
				refs = append(refs, ref)
				slog.Debug("new ref", slog.String("ref", ref.String()))
			}

			if node.Parent != nil {
				node.Parent.InsertBefore(a, node)
				slog.Debug("insert anchor", slog.String("a", anyJson(a)), slog.String("node", anyJson(node)))
			} else {
				slog.Debug("node has no parent", slog.Any("node", anyJson(node)))
			}
		}
	}

	return refs
}

func (d *Dash) insertDB() error {
	for _, ref := range d.refs {
		_, err := d.db.Exec(`INSERT OR IGNORE INTO searchIndex(name, type, path) VALUES (?,?,?)`, ref.name, ref.etype, ref.href())
		if err != nil {
			return errors.Wrapf(err, "insert searchIndex %s %s %s", ref.name, ref.etype, ref.href())
		}
		slog.Debug("insert ref to db", slog.String("name", ref.name), slog.String("type", ref.etype), slog.String("href", ref.href()))
	}
	return nil
}

func (d Dash) writeHTML(path string, doc *html.Node) error {
	// Clean the path and check for traversal
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "../") || strings.Contains(cleanPath, "/../") {
		return errors.Errorf("path traversal detected: %s", path)
	}

	absoultePath := filepath.Join(d.tree.Documents(), cleanPath)

	// Verify absoultePath is still within Documents
	if !strings.HasPrefix(absoultePath, d.tree.Documents()) {
		return errors.Errorf("path escapes Documents directory: %s", path)
	}

	dirname := filepath.Dir(absoultePath)
	err := os.MkdirAll(dirname, 0755)
	if err != nil {
		return errors.Wrapf(err, "dirname %s", dirname)
	}

	// Write to temp file first for atomicity
	tempPath := absoultePath + ".tmp"
	f, err := os.Create(tempPath)
	if err != nil {
		return errors.Wrapf(err, "Create temp %s", tempPath)
	}
	defer f.Close()

	err = html.Render(f, doc)
	f.Close()

	if err != nil {
		os.Remove(tempPath) // Clean up temp on error
		return errors.Wrapf(err, "Render doc, %s", absoultePath)
	}

	// Atomic rename
	if err := os.Rename(tempPath, absoultePath); err != nil {
		os.Remove(tempPath)
		return errors.Wrapf(err, "Rename to %s", absoultePath)
	}

	return nil
}

func text(node *html.Node) string {
	var b bytes.Buffer
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		} else if c.Type == html.ElementNode {
			b.WriteString(text(c))
		}
	}
	return strings.TrimSpace(b.String())
}

func attr(node *html.Node, key string) string {
	for _, a := range node.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func newLinkFromNode(node *html.Node) *html.Node {
	if node == nil {
		return nil
	}

	val := attr(node, "name")
	return &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Link,
		Data:     atom.Link.String(),
		Attr: []html.Attribute{
			{Key: "href", Val: val},
		},
	}
}

func newA(name, etype string, level int) *html.Node {
	name = url.PathEscape(name)
	val := fmt.Sprintf("//dash_ref_%s/%s/%s/%d", name, etype, name, level)
	return &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.A,
		Data:     atom.A.String(),
		Attr: []html.Attribute{
			{Key: "class", Val: "dashAnchor"},
			{Key: "name", Val: val},
		},
	}
}

func pathRelativeToRoot(path string) string {
	depth := len(strings.Split(path, "/")) - 1

	parents := make([]string, 0)
	for i := 0; i < depth; i++ {
		parents = append(parents, "..")
	}

	prefix := strings.Join(parents, "/")
	return prefix
}

func relativeURL(prefix string, u *url.URL, suffix string) string {
	cu := *u
	if !strings.HasSuffix(cu.Path, suffix) {
		cu.Path += suffix
	}
	str := cu.String()
	str, _ = strings.CutPrefix(str, cu.Scheme+":/")
	return prefix + str
}

func localPathOfURL(u *url.URL, suffix string) string {
	if strings.HasSuffix(u.Path, suffix) {
		return u.Host + u.Path
	}
	return u.Host + u.Path + suffix
}

func anyJson(v any) string {
	s, _ := json.Marshal(v)
	return string(s)
}
