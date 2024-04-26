package dashdog

// <?xml version="1.0" encoding="UTF-8"?>
// <plist version="1.0">
//     <dict>
//         <key>CFBundleIdentifier</key>
//         <string>godoc</string>
//         <key>CFBundleName</key>
//         <string>go-resty/resty/v2</string>
//         <key>DocSetPlatformFamily</key>
//         <string>godoc</string>
//         <key>dashIndexFilePath</key>
//         <string>pkg.go.dev/github.com/go-resty/resty/v2.html</string>
//         <key>isJavaScriptEnabled</key>
//         <true />
//         <key>isDashDocset</key>
//         <true />
//         <key>DashDocSetPlayURL</key>
//         <string>{{.DashDocSetPlayURL}}</string>
//     </dict>
// </plist>

const plistTpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>{{.CFBundleIdentifier}}</string>
	<key>CFBundleName</key>
	<string>{{.CFBundleName}}</string>
	<key>DocSetPlatformFamily</key>
	<string>{{.DocSetPlatformFamily}}</string>
	<key>dashIndexFilePath</key>
	<string>{{.DashIndexFilePath}}</string>
	<key>isDashDocset</key>
	<true/>
	{{- if .IsJavaScriptEnabled }}
	<key>isJavaScriptEnabled</key>
	<true />
	{{- end }}
	{{- if .DashDocSetPlayURL }}
	<key>DashDocSetPlayURL</key>
	<string>{{.DashDocSetPlayURL}}</string>
	{{- end }}
	{{- if .DashDocSetDefaultFTSEnabled }}
	<key>DashDocSetDefaultFTSEnabled</key>
	<true/>
	{{- end }}
</dict>
</plist>
`

type InfoPlistModel struct {
	CFBundleIdentifier          string
	CFBundleName                string
	DocSetPlatformFamily        string
	DashIndexFilePath           string
	DashDocSetPlayURL           string
	IsJavaScriptEnabled         bool
	DashDocSetDefaultFTSEnabled bool
}
