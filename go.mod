module github.com/v8tix/beecore-store-v2

go 1.26

//replace github.com/v8tix/beecore-eda v1.2.0 => ../beecore-eda

//replace github.com/v8tix/beecore-auth-v3-mod v1.0.5 => ../beecore-auth-v3-mod

replace github.com/v8tix/beecore-http v0.1.0 => ../beecore-http

//replace github.com/v8tix/kawa v1.0.2 => ../../kawa

require (
	github.com/fatih/color v1.19.0
	github.com/go-playground/form/v4 v4.3.0
	golang.org/x/crypto v0.54.0
	golang.org/x/exp v0.0.0-20260718201538-764159d718ef
	golang.org/x/text v0.40.0
)

require (
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
