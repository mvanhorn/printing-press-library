module github.com/mvanhorn/printing-press-library/library/marketing/traffic-intel

go 1.25.11

require (
	github.com/mvanhorn/printing-press-library/library/internal/intelcli v0.0.0
	github.com/spf13/cobra v1.9.1
)

replace github.com/mvanhorn/printing-press-library/library/internal/intelcli => ../../internal/intelcli

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
)
