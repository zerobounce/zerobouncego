// Module path and semver tags must stay aligned (Go semantic import versioning):
//   - v0.x / v1.x → module github.com/zerobounce/zerobouncego (no /v2)
//   - v2.x+       → module github.com/zerobounce/zerobouncego/v2  (this file)
// If you tag v2.0.0 or higher while the module line omits /v2, proxy.golang.org
// will not serve that version (404). See README § Publish and sdk-docs/pkg-go-dev/.
module github.com/zerobounce/zerobouncego/v2

go 1.16

require (
	github.com/jarcoal/httpmock v1.4.1
	github.com/joho/godotenv v1.5.1
	github.com/stretchr/testify v1.11.1
	gopkg.in/guregu/null.v4 v4.0.0
)
