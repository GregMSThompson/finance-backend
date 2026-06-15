package dto

// AppleAppSiteAssociation is the JSON Apple expects at
// /.well-known/apple-app-site-association to validate universal links.
type AppleAppSiteAssociation struct {
	Applinks AppleAppLinks `json:"applinks"`
}

// AppleAppLinks holds the universal-link configuration block.
// Apps is a legacy field that Apple's spec requires to be present (and empty).
type AppleAppLinks struct {
	Apps    []string             `json:"apps"`
	Details []AppleAppLinkDetail `json:"details"`
}

// AppleAppLinkDetail maps an app ID (TEAMID.bundleid) to the URL paths that
// should open the app via universal links.
type AppleAppLinkDetail struct {
	AppID string   `json:"appID"`
	Paths []string `json:"paths"`
}
