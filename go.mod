module github.com/uyuni-project/minima

go 1.26.5

require (
	github.com/ProtonMail/go-crypto v1.4.1
	github.com/aws/aws-sdk-go v1.55.8
	github.com/klauspost/compress v1.19.0
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.10.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/cloudflare/circl v1.6.4 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/ProtonMail/go-crypto => github.com/pgpkeys-eu/go-crypto v1.4.1-pgpkeys
