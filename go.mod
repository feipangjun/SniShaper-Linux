module snishaper

go 1.25.5

require (
	github.com/miekg/dns v1.1.72
	github.com/quic-go/quic-go v0.59.0
	github.com/refraction-networking/utls v1.8.2
	golang.org/x/net v0.53.0
	golang.org/x/sys v0.43.0
)

require (
	github.com/andybalholm/brotli v1.0.6 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/mod v0.34.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/tools v0.43.0 // indirect
)

replace github.com/refraction-networking/utls => ./utls
