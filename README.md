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

To test sending with DLR do

```bash
curl "http://localhost:8080/bulk_server?type=text&user=john&password=smith&sender=johnny&receiver=4178123456&text=Hello&dlr-mask=19&dlr-url=http%3A%2F%2Flocalhost:8080%2Fdlr_test%3Fmsgid%3D%25U%26dlr-event%3D%25d%26sender%3D%25s%26receiver%3D%25r%26error_code%3D%25e%26error_msg%3D%25E%26part_num%3D%25p%26total_parts%3D%25P"
```