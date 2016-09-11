package main

import (
	"log"
	"net/http"
	"net/url"
	"fmt"
	"os/exec"
	"strings"
	//"io"
)

func isEmpty(form *url.Values, fieldName string) bool {
	return len(form.Get(fieldName)) == 0
}

func getUUID() string {
    uuid, err := exec.Command("uuidgen").Output()
    if err != nil {
        log.Fatal(err)
    }
    //var cutset string
    //log.Printf("uuid: [%s]", uuid);
    return strings.TrimSpace(string(uuid))
}

// serveBulkServer handles bulk gate requests
func serveBulkServer(hub *Hub, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	log.Print("Bulk server request raw: ",r.Form)

	//check params
	if isEmpty(&r.Form, "sender") || isEmpty(&r.Form, "receiver") || isEmpty(&r.Form, "text"){
	    log.Println("Bulk request invalid params")
	    http.Error(w, "ERR 110", 420)
	    return
	    //io.WriteString(w, "ERR 110")
	}
	hub.broadcastMessageParams(r.Form.Get("sender"), r.Form.Get("receiver"), r.Form.Get("text"));

	messageId := getUUID()
	smsParts := 1

	//close http conn. and flush
	w.WriteHeader(http.StatusOK);
	w.Write([]byte(fmt.Sprintf("OK %s %v", messageId, smsParts)))

	log.Printf("Valid request and replied: OK %s %v\n", messageId, smsParts);	
}