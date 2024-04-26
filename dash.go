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

	css "github.com/andybalholm/cascadia"
	"github.com/go-resty/resty/v2"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type fetchItem struct {
	u            *url.URL
	localPath    string
	level        int
	needPopulate bool
}

func (item fetchItem) String() string {
	return fmt.Sprintf("url:%s localPath:%s level:%d needPopulate:%v", item.u.String(), item.localPath, item.level, item.needPopulate)
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
}

type Reference struct {
	name  string
	etype string
	href  string
}

func (r Reference) String() string {
	return fmt.Sprintf("name:%s type:%s href:%s", r.name, r.etype, r.href)
}

func NewDash(config Config) (*Dash, error) {
	d := &Dash{
		httpClient: resty.New(),
		tree:       newDocTree(config.Name),
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
	if config.SubPathBundleNameReplace.Pattern != "" {
		d.subPathBundleNameRegex, err = regexp.Compile(config.SubPathBundleNameReplace.Pattern)
		if err != nil {
			return nil, errors.Wrapf(err, "regexp.Compile SubPathBundleNameReplace.Pattern %s", config.SubPathBundleNameReplace.Pattern)
		}
	}

	return d, nil
}

func (d *Dash) Build() error {
	// remove old data if exist
	if err := d.tree.Rm(); err != nil {
		return errors.Wrapf(err, "rm")
	}
	slog.Info("remove docpath", slog.String("path", d.tree.Documents()))

	// Create the docset folder
	if err := d.tree.Mkdir(); err != nil {
		return errors.Wrapf(err, "mkdir")
	}
	slog.Info("mkdir", slog.String("path", d.tree.Documents()))

	// create sqlite index
	if err := d.createDB(); err != nil {
		return errors.Wrapf(err, "createDB")
	}
	slog.Info("open db", slog.String("path", d.tree.DB()))
	defer func() {
		if d.db != nil {
			d.db.Close()
		}
	}()

	u, err := url.Parse(d.config.URL)
	if err != nil {
		return errors.Wrapf(err, "Parse %s", d.config.URL)
	}
	slog.Info("parse url", slog.String("url", d.config.URL))

	localPath := localPathOfURL(u, ".html")
	d.indexFilePath = localPath

	// create the info.plist
	if err := d.infoPlist(); err != nil {
		return errors.Wrap(err, "infoPlist")
	}
	slog.Info("create info.plist", slog.String("path", d.tree.InfoPlist()))

	item := &fetchItem{
		u:            u,
		localPath:    localPath,
		level:        0,
		needPopulate: true,
	}
	d.pushItem(item)
	slog.Info("push root item", slog.String("item", item.String()))

	// populate the sqllite index
	for !d.queueEmpty() {
		item, ok := d.popItem()
		if !ok {
			slog.Info("popItem finish")
			break
		}

		slog.Debug("popItem", slog.String("item", item.String()))
		if err := d.populateData(item); err != nil {
			return errors.Wrapf(err, "populateData")
		}
	}

	return nil
}

func (d Dash) queueEmpty() bool {
	return len(d.fetchQueue) == 0
}

func (d *Dash) pushItem(item *fetchItem) {
	d.fetchQueue = append(d.fetchQueue, item)
}

func (d *Dash) popItem() (*fetchItem, bool) {
	if d.queueEmpty() {
		return nil, false
	}
	item := d.fetchQueue[0]
	d.fetchQueue = d.fetchQueue[1:]
	return item, true
}

func (d Dash) saveFile(localPath string, body []byte) error {
	dirname := filepath.Dir(localPath)
	absPath := filepath.Join(d.tree.Documents(), dirname)
	err := os.MkdirAll(absPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "MkdirAll %s", absPath)
	}
	slog.Debug("mkdir", slog.String("path", absPath))

	absPath = filepath.Join(d.tree.Documents(), localPath)
	_, err = os.Stat(absPath)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return errors.Wrapf(err, "Stat file %s", localPath)
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

	t, err := template.New(d.config.Name).Parse(plistTpl)
	if err != nil {
		return errors.Wrap(err, "Parse plistTpl")
	}

	file, err := os.Create(d.tree.InfoPlist())
	if err != nil {
		return errors.Wrapf(err, "Create %s", d.tree.InfoPlist())
	}

	err = t.Execute(file, m)
	return errors.Wrapf(err, "Execute m:%+v", m)
}

func (d *Dash) createDB() error {
	dbname := d.tree.DB()

	db, err := sql.Open("sqlite3", dbname)
	if err != nil {
		return errors.Wrapf(err, "Open db, %s", dbname)
	}

	// DELETE FROM <table>;
	// UPDATE SQLITE_SEQUENCE SET seq = 0 WHERE name = '<table>';
	// VACUUM;

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS searchIndex(id INTEGER PRIMARY KEY, name TEXT, type TEXT, path TEXT)`); err != nil {
		return errors.Wrapf(err, "create table")
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS anchor ON searchIndex (name, type, path)`); err != nil {
		return errors.Wrapf(err, "create unique index")
	}
	if _, err := db.Exec(`DELETE FROM searchIndex WHERE 1=1`); err != nil {
		return errors.Wrapf(err, "delete rows")
	}

	d.db = db
	return nil
}

func (d *Dash) populateData(item *fetchItem) error {
	urlStr := item.u.String()

	checkPath := item.u.Host + item.u.Path
	if d.downloaded[checkPath] {
		slog.Debug("downloaded", slog.String("path", checkPath))
		return nil
	}
	d.downloaded[checkPath] = true

	resp, err := d.httpClient.R().Get(urlStr)
	if err != nil {
		return errors.Wrapf(err, "get %s", urlStr)
	}

	if resp.StatusCode() != http.StatusOK {
		return errors.Errorf("%s status %d", urlStr, resp.StatusCode())
	}

	if !item.needPopulate {
		err = d.saveFile(item.localPath, resp.Body())
		slog.Debug("download resource", slog.String("localPath", item.localPath), slog.String("url", item.u.String()))
		return errors.Wrapf(err, "saveFile")
	}

	slog.Info("populateData url", slog.String("url", urlStr))

	u := item.u

	r := bytes.NewReader(resp.Body())
	doc, err := html.Parse(r)
	if err != nil {
		return errors.Wrapf(err, "Parse html of %s", urlStr)
	}

	// remove nodes before fetch resource
	// and then we can not download the resource we don't need
	d.removeNode(doc)
	slog.Info("removeNode", slog.String("item", item.String()))
	d.setAttr(doc)
	slog.Info("setAttr", slog.String("item", item.String()))

	err = d.fetchResource(u, doc, item.level)
	if err != nil {
		return errors.Wrapf(err, "fetchResource %s", urlStr)
	}
	slog.Info("fetchResource", slog.String("item", item.String()))

	subRefs := d.insertAnchor(u, item.localPath, doc)
	slog.Info("insertAnchor", slog.String("item", item.String()))

	d.insertOnlineRedirection(doc, urlStr)
	slog.Info("insertOnlineRedirection", slog.String("url", urlStr))

	d.insertLink(doc)
	slog.Info("insertLink", slog.String("item", item.String()))

	err = d.writeHTML(item.localPath, doc)
	if err != nil {
		return errors.Wrap(err, "writeHTML")
	}
	slog.Info("write html", slog.String("path", item.localPath))

	refs := []*Reference{
		{
			name:  d.bundleNameOfPath(item.u.Path),
			etype: "Package",
			href:  item.localPath,
		},
	}
	slog.Info("insert package", slog.String("name", refs[0].name), slog.String("type", refs[0].etype), slog.String("href", refs[0].href))
	refs = append(refs, subRefs...)

	d.insertDB(refs)
	slog.Info("insertDB", slog.String("item", item.String()), slog.Int("len(refs)", len(refs)))

	return nil
}

func (d Dash) bundleNameOfPath(path string) string {
	if path == "" {
		return d.config.Plist.CFBundleName
	}
	if d.config.SubPathBundleNameReplace.Replace == "" {
		return path
	}
	if !d.subPathBundleNameRegex.MatchString(path) {
		return d.config.Plist.CFBundleName
	}

	return d.subPathBundleNameRegex.ReplaceAllString(path, d.config.SubPathBundleNameReplace.Replace)
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
		slog.Info("insert online redirection", slog.Any("node", node))
	}
}

func (d Dash) removeNode(doc *html.Node) {
	for _, sel := range d.config.Page.RemoveNodeSelector {
		nodeSelector := css.MustCompile(sel)
		nodes := nodeSelector.MatchAll(doc)
		for _, node := range nodes {
			node.Parent.RemoveChild(node)
			slog.Info("remove node", slog.String("selector", sel), slog.Any("node", anyJson(node)))
		}
	}
}

func (d Dash) setAttr(doc *html.Node) {
	for _, sattr := range d.config.Page.SetAttrs {
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
					slog.Info("set attr", slog.Any("sattr", anyJson(sattr)), slog.Any("node", anyJson(node)))
					break
				}
			}
			if !found {
				slog.Info("add attr", slog.Any("sattr", anyJson(sattr)), slog.Any("node", anyJson(node)))
				node.Attr = append(node.Attr, html.Attribute{
					Key: sattr.Attr.Key,
					Val: sattr.Attr.Value,
				})
			}
		}
	}
}

func (d *Dash) fetchResource(ourl *url.URL, doc *html.Node, level int) error {
	prefix := pathRelativeToRoot(ourl.Path)

	resourceSelector := css.MustCompile("*[href],*[src]")
	nodes := resourceSelector.MatchAll(doc)
	for _, node := range nodes {
		for i, attr := range node.Attr {
			if attr.Key != "href" && attr.Key != "src" {
				continue
			}

			u, err := url.Parse(attr.Val)
			if err != nil {
				return errors.Wrapf(err, "Parse %s", attr.Val)
			}

			if u.Host == "" {
				u.Host = ourl.Host
			}
			if u.Scheme == "" {
				u.Scheme = ourl.Scheme
			}

			// not a atom.A => set a relative url, push to queue
			// is atom.A
			//   from a difference site => set the whole url
			//   from the same site, same url => set the relative url
			//   from the same site, not a sub page => set the whole url
			//   from the same site, is a sub page => set the relative url, push to queue

			if node.DataAtom != atom.A {
				suffix := ""
				node.Attr[i].Val = relativeURL(prefix, u, suffix)
				item := &fetchItem{
					u:            u,
					localPath:    localPathOfURL(u, suffix),
					level:        level,
					needPopulate: false,
				}
				d.pushItem(item)
				slog.Debug("pushItem", slog.String("item", item.String()))
				continue
			}

			if ourl.Host != u.Host {
				node.Attr[i].Val = u.String()
				continue
			}

			if ourl.Path == u.Path {
				node.Attr[i].Val = relativeURL(prefix, u, "")
				continue
			}

			if !strings.HasPrefix(u.Path, ourl.Path) {
				node.Attr[i].Val = u.String()
				continue
			}

			if level+1 <= d.config.Depth-1 && d.pathMatchRegex(u.Path) {
				suffix := ".html"
				node.Attr[i].Val = relativeURL(prefix, u, suffix)
				item := &fetchItem{
					u:            u,
					localPath:    localPathOfURL(u, suffix),
					level:        level + 1,
					needPopulate: true,
				}
				d.pushItem(item)
				slog.Debug("pushItem", slog.String("item", item.String()))
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
			slog.Info("insert link", slog.String("href", attr(link, "href")))
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

			if name == "" {
				slog.Info("name is empty", slog.Any("node", anyJson(node)))
				continue
			}

			a := newA(name, sel.Type, sel.Level)

			if !sel.AnchorOnly {
				anchor := attr(a, "name")
				// CbunelName TODO
				href := fmt.Sprintf(`<dash_entry_name=%s><dash_entry_originalName=%s.%s><dash_entry_menuDescription=%s>%s#%s`, name, d.bundleNameOfPath(u.Path), name, d.config.Plist.CFBundleName, localPath, anchor)
				ref := &Reference{
					name:  name,
					etype: sel.Type,
					href:  href,
				}
				refs = append(refs, ref)
				slog.Debug("new ref", slog.String("ref", ref.String()))
			}

			node.Parent.InsertBefore(a, node)
			slog.Debug("insert anchor", slog.String("a", anyJson(a)), slog.String("node", anyJson(node)))
		}
	}

	return refs
}

func (d *Dash) insertDB(refs []*Reference) {
	for _, ref := range refs {
		d.db.Exec(`INSERT OR IGNORE INTO searchIndex(name, type, path) VALUES (?,?,?)`, ref.name, ref.etype, ref.href)
		slog.Info("insert ref to db", slog.String("name", ref.name), slog.String("type", ref.etype), slog.String("href", ref.href))
	}
}

func (d Dash) writeHTML(path string, doc *html.Node) error {
	absoultePath := filepath.Join(d.tree.Documents(), path)
	dirname := filepath.Dir(absoultePath)
	err := os.MkdirAll(dirname, 0755)
	if err != nil {
		return errors.Wrapf(err, "dirname %s", dirname)
	}

	f, err := os.Create(absoultePath)
	if err != nil {
		return errors.Wrapf(err, "Create %s", absoultePath)
	}
	defer f.Close()

	err = html.Render(f, doc)
	return errors.Wrapf(err, "Render doc, %s", absoultePath)
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
