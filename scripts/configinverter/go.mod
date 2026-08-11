module github.com/GuanceCloud/dd-trace-go/v2/scripts/configinverter

go 1.25.0

require (
	github.com/GuanceCloud/dd-trace-go/v2 v2.10.1-ext
	github.com/dave/jennifer v1.7.1
)

require golang.org/x/mod v0.35.0 // indirect

replace github.com/GuanceCloud/dd-trace-go/v2 => ../..
