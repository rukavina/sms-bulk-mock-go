package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"os/exec"
	"strings"
	"time"
)

//number which simulate bulk bug - to panic
const panicNumber = "41764986185"

//number which simulate bulk bug - to response too late
const timeoutNumber = "41764986186"

//BulkRequestAuth is auth related embed struct
type BulkRequestAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

//BulkRequest is new SMS request
type BulkRequest struct {
	Type     string          `json:"type"`
	Auth     BulkRequestAuth `json:"Auth"`
	Sender   string          `json:"sender"`
	Receiver string          `json:"receiver"`
	Dcs      string          `json:"dcs"`
	Text     string          `json:"text"`
	DlrMask  int             `json:"dlrMask"`
	DlrURL   string          `json:"dlrUrl"`
}

//BulkResultSuccess is success response
type BulkResultSuccess struct {
	MsgID    string `json:"msgId"`
	NumParts int    `json:"numParts"`
}

//BulkError is error response
type BulkError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

//BulkResultError is response error wrapper
type BulkResultError struct {
	Error BulkError `json:"error"`
}

//BulkDlr is dlr update
type BulkDlr struct {
	MsgID        string `json:"msgId"`
	Event        string `json:"event"`
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	PartNum      int    `json:"partNum"`
	TotalParts   int    `json:"totalParts"`
	NumParts     int    `json:"numParts"`
	AccountName  string `json:"accountName"`
}

func isEmpty(fieldValue string) bool {
	return len(fieldValue) == 0
}

func getUUID() string {
	uuid, err := exec.Command("uuidgen").Output()
	if err != nil {
		log.Fatal(err)
	}

	return strings.TrimSpace(string(uuid))
}

func jsonResult(w http.ResponseWriter, httpCode int, jsonData interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(jsonData)
}

func makeErrorResult(errorCode string, message string) BulkResultError {
	return BulkResultError{
		Error: BulkError{
			Code:    errorCode,
			Message: message,
		},
	}
}

var messageCounter int

// serveBulkServer handles bulk gate requests
func serveBulkServer(hub *Hub, w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, true)
	log.Print("Bulk server request raw body: ", string(dump))
	decoder := json.NewDecoder(r.Body)
	var reqJSON BulkRequest
	err = decoder.Decode(&reqJSON)
	if err != nil {
		log.Println("Bulk request invalid", err)
		jsonResult(w, 420, makeErrorResult("109", "Format of text/content parameter iswrong."))
		return
	}
	messageCounter++
	//send throttle error just for fun
	if messageCounter > 500 {
		messageCounter = 0
		jsonResult(w, 420, makeErrorResult("105", "Too many messages submitted withing short period of time. Resend later."))
		return
	}
	//check params
	if isEmpty(reqJSON.Sender) || isEmpty(reqJSON.Receiver) || isEmpty(reqJSON.Text) {
		log.Println("Bulk request invalid params")
		jsonResult(w, 420, makeErrorResult("110", "Mandatory parameter is missing"))
		return
	}
	hub.broadcastMessageParams(reqJSON.Sender, reqJSON.Receiver, reqJSON.Text)

	messageID := getUUID()
	smsParts := getNumberOfSMSsegments(reqJSON.Text, 6)

	//simulate long timeout
	if reqJSON.Receiver == timeoutNumber {
		time.Sleep(time.Second * 45)
	}

	//close http conn. and flush
	if reqJSON.Receiver != panicNumber {
		resultSuccess := BulkResultSuccess{
			MsgID:    messageID,
			NumParts: smsParts,
		}
		jsonResult(w, 202, resultSuccess)

		log.Printf("Valid request and replied: OK %s %v\n", messageID, smsParts)
	}

	//is there DLR handler?
	if isEmpty(reqJSON.DlrURL) {
		return
	}

	notificationDlr := BulkDlr{
		MsgID:        messageID,
		Event:        "DELIVERED",
		ErrorCode:    0,
		ErrorMessage: "",
		PartNum:      1,
		TotalParts:   smsParts,
		NumParts:     smsParts,
		AccountName:  reqJSON.Auth.Username,
	}

	//send dlr as go routine
	go sendDlr(reqJSON, notificationDlr)

	if reqJSON.Receiver == panicNumber {
		panic("Panic on receiver [" + reqJSON.Receiver + "]")
	}
}

// send dlr
func sendDlr(reqJSON BulkRequest, notificationDlr BulkDlr) {
	log.Println("Sending DLR notification to ", reqJSON.DlrURL)
	//give a timeout
	time.Sleep(time.Second * 2)
	dlrBytes, err := json.Marshal(notificationDlr)
	if err != nil {
		log.Println("DLR notification err:", err)
		return
	}
	req, err := http.NewRequest("POST", reqJSON.DlrURL, bytes.NewBuffer(dlrBytes))
	req.Header.Set("Content-Type", "application/json")

	//disable certificate verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   time.Second * 10,
		Transport: tr,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("DLR notification err:", err)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		if r := recover(); r != nil {
			log.Println("DLR notification recovered:", r)
		}
	}()
}

// serveTestDlrHandler handles test dlrs
func serveTestDlrHandler(w http.ResponseWriter, r *http.Request) {
	dump, _ := httputil.DumpRequest(r, true)
	log.Print("Bulk server DLR request raw:", string(dump))
}

func isGsm7bit(text string) bool {
	gsm7bitChars := "\\@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞÆæßÉ !\"#¤%%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà^{}[~]|€"

	for _, c := range text {
		if !strings.ContainsRune(gsm7bitChars, c) {
			return false
		}
	}
	return true
}

func getNumberOfSMSsegments(text string, maxSegments int) int {
	totalSegment := 0
	textLen := len(text)
	if textLen == 0 {
		return 0 //I can see most mobile devices will not allow you to send empty sms, with this check we make sure we don't allow empty SMS
	}
	//UCS-2 Encoding (16-bit)
	singleMax := 70
	concatMax := 67
	if isGsm7bit(text) { //7-bit
		singleMax = 160
		concatMax = 153
	}
	if textLen <= singleMax {
		totalSegment = 1
	} else {
		totalSegment = textLen / concatMax
	}
	if totalSegment > maxSegments {
		return 0 //SMS is very big.
	}
	return totalSegment
}
