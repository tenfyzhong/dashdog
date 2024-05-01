package dashdog

type IndexNameType int

const (
	IndexNameTypeText     IndexNameType = 0 // use node text
	IndexNameTypeAttr     IndexNameType = 1 // use the value of the attr as the index name
	IndexNameTypeConstant IndexNameType = 2 // use Value as the index name
)

type IndexName struct {
	Type  IndexNameType
	Value string
}

type IndexRow struct {
	Selector   string    `yaml:"selector"`    // the selector to select nodes which should be match the selector
	Type       string    `yaml:"type"`        // The dash type for the match node
	Name       IndexName `yaml:"name"`        // indices how to get the index name
	Level      int       `yaml:"level"`       // TOC level
	AnchorOnly bool      `yaml:"anchor_only"` // only insert anchor node, do not insert into table
}

type Plist struct {
	CFBundleIdentifier          string `yaml:"cfbundle_identifier"`
	CFBundleName                string `yaml:"cfbundle_name"`
	DocSetPlatformFamily        string `yaml:"doc_set_platform_family"`
	DashDocSetPlayURL           string `yaml:"dash_doc_set_play_url"`
	IsJavaScriptEnabled         bool   `yaml:"is_java_script_enabled"`
	DashDocSetDefaultFTSEnabled bool   `yaml:"dash_doc_set_default_ftsenabled"`
}

type Index struct {
	IndexRows []IndexRow `yaml:"index_rows"`
}

type Attr struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

type SelectAttr struct {
	Selector string `yaml:"selector"`
	Attr     Attr   `yaml:"attr"`
}

type Page struct {
	RemoveNodeSelector []string     `yaml:"remove_node_selector"`
	SetAttrs           []SelectAttr `yaml:"set_attrs"`
}

type SubPathBundleNameReplace struct {
	Pattern string `yaml:"pattern"` // a pattern match the path of url
	Replace string `yaml:"replace"` // a pattern to replace the source path
}

type Config struct {
	Path                     string                   `yaml:"path"`           // The path to generate docset, it will be make if not exist
	Name                     string                   `yaml:"name"`           // docset name
	URL                      string                   `yaml:"url"`            // the html url to populate
	Plist                    Plist                    `yaml:"plist"`          // config info.plit
	Index                    Index                    `yaml:"index"`          // sqlite index
	Page                     Page                     `yaml:"page"`           // html page modify
	Depth                    int                      `yaml:"depth"`          // max depth to process
	SubPathRegex             string                   `yaml:"sub_path_regex"` // which sub page will be process if the path match the regex
	SubPathBundleNameReplace SubPathBundleNameReplace `yaml:"sub_path_bundle_name_replace"`
}
