package main

import (
	"log"
	"net/http"
	"net/url"
	"fmt"
	"os/exec"
	"strings"
	"strconv"
)

func isEmpty(form *url.Values, fieldName string) bool {
	return len(form.Get(fieldName)) == 0
}

func getUUID() string {
    uuid, err := exec.Command("uuidgen").Output()
    if err != nil {
        log.Fatal(err)
    }

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

	//is there DLR handler?
	if(isEmpty(&r.Form, "dlr-url")){
		return;
	}	

	//values map
	values := map[string]string{
		"messageId": messageId,
		"dlrEvent": "1",
		"errorCode": "0",
		"errorDesc": "",
		"smsParts": strconv.Itoa(smsParts),	
	}

	//send dlr as go routine
	go sendDlr(r.Form.Get("dlr-url"), values, r.Form)
}

// send dlr 
func sendDlr(dlrUrlPattern string, values map[string]string, form url.Values){
	dlrVars := map[string]string{
	    "%U": values["messageId"],	//Message ID	Message ID as returned when message is sent, see here
	    "%d": values["dlrEvent"],
	    "%s": form.Get("sender"),	//Sender	
	    "%r": form.Get("receiver"),	//Receiver	
	    "%e": values["errorCode"],	//Error code	26
	    "%E": values["errorDesc"],	//Error description	Unknown subscriber
	    "%A": form.Get("user"),	//Account name used for submission.	YOUR_USERNAME
	    "%p": "0",	//Part number [0 to total_parts-1]	1
	    "%P": values["smsParts"],	//Total number of parts	3
	}
	for key, value := range dlrVars {
		dlrUrlPattern = strings.Replace(dlrUrlPattern, key, value, -1)
	}
	http.Get(dlrUrlPattern)
}

// serveTestDlrHandler handles test dlrs
func serveTestDlrHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	log.Print("Bulk server DLR request raw: ",r.Form)
}