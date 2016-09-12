# SMS Bulk Mock test tool - v4, golang

This is a mock server for HORISEN AG bulk sms API: https://www.horisen.com/en/help/api-manuals/bulk-http


## Running the example

The example requires a working Go development environment. The [Getting
Started](http://golang.org/doc/install) page describes how to install the
development environment.

Once you have Go up and running, you can download, build and run the example
using the following commands.

```bash
git clone git@github.com:rukavina/sms-bulk-mock-go.git
cd sms-bulk-mock-go
go get github.com/gorilla/websocket
cd public
bower install
cd ..
go run *.go
```

To use the chat example, open http://localhost:8080/ in your browser.

Quick command line test:

```bash
CONTENT='{"type":"text","auth":{"username":"testuser","password":"testpassword"},"sender":"BulkTest","receiver":"41787078880","dcs":"GSM", "text":"This is test message","dlrMask":19,"dlrUrl":"http://localhost:8080/dlr_test"}'
curl -L "http://localhost:8080/bulk_server" -XPOST -d "$CONTENT"
```

## Run on different port

In order to run app on a port other than `8080` you can provide additional parameter `--addr=:port`, eg:

```bash
go run *.go --addr=:9000
```