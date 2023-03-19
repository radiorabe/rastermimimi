# rastermimimi

Schaut sich die diversen Programmraster Quellen an und macht mimimi.

## Usage

```
Usage of rastermimimi:
  -duration int
        How far ahead to check in days (default 60)
  -libretime string
        URL of Libretime Instance to compare (default "https://airtime.service.int.rabe.ch")
  -listen-addr string
        listen address (default ":8080")
  -tls-cert string
        path to TLS cert
  -tls-key string
        path to TLS key
  -url string
        URL of RaBe Website (default "https://rabe.ch")
```

## Development

Run mimimi to test changes:
```
go run .
```
