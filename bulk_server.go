package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"os/exec"
	"strings"
	"time"
)

const panicNumber = "41764986185"

type BulkRequestAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type BulkRequest struct {
	Type     string          `json:"type"`
	Auth     BulkRequestAuth `json:"Auth"`
	Sender   string          `json:"sender"`
	Receiver string          `json:"receiver"`
	Dcs      string          `json:"dcs"`
	Text     string          `json:"text"`
	DlrMask  int             `json:"dlrMask"`
	DlrUrl   string          `json:"dlrUrl"`
}

type BulkResultSuccess struct {
	MsgId    string `json:"msgId"`
	NumParts int    `json:"numParts"`
}

type BulkError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type BulkResultError struct {
	Error BulkError `json:"error"`
}

type BulkDlr struct {
	MsgId        string `json:"msgId"`
	Event        string `json:"event"`
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	PartNum      int    `json:"partNum"`
	TotalParts   int    `json:"totalParts"`
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

var messageCounter int = 0

// serveBulkServer handles bulk gate requests
func serveBulkServer(hub *Hub, w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, true)
	log.Print("Bulk server request raw body: ", string(dump))
	decoder := json.NewDecoder(r.Body)
	var reqJson BulkRequest
	err = decoder.Decode(&reqJson)
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
	if isEmpty(reqJson.Sender) || isEmpty(reqJson.Receiver) || isEmpty(reqJson.Text) {
		log.Println("Bulk request invalid params")
		jsonResult(w, 420, makeErrorResult("110", "Mandatory parameter is missing"))
		return
	}
	hub.broadcastMessageParams(reqJson.Sender, reqJson.Receiver, reqJson.Text)

	messageId := getUUID()
	smsParts := getNumberOfSMSsegments(reqJson.Text, 6)

	//close http conn. and flush
	if reqJson.Receiver != panicNumber {
		resultSuccess := BulkResultSuccess{
			MsgId:    messageId,
			NumParts: smsParts,
		}
		jsonResult(w, 202, resultSuccess)

		log.Printf("Valid request and replied: OK %s %v\n", messageId, smsParts)
	}

	//is there DLR handler?
	if isEmpty(reqJson.DlrUrl) {
		return
	}

	notificationDlr := BulkDlr{
		MsgId:        messageId,
		Event:        "DELIVERED",
		ErrorCode:    0,
		ErrorMessage: "",
		PartNum:      0,
		TotalParts:   smsParts,
		AccountName:  reqJson.Auth.Username,
	}

	//send dlr as go routine
	go sendDlr(reqJson, notificationDlr)

	if reqJson.Receiver == panicNumber {
		panic("Panic on receiver [" + reqJson.Receiver + "]")
	}
}

// send dlr
func sendDlr(reqJson BulkRequest, notificationDlr BulkDlr) {
	log.Println("Sending DLR notification to ", reqJson.DlrUrl)
	//give a timeout
	time.Sleep(time.Second * 2)
	dlrBytes, err := json.Marshal(notificationDlr)
	if err != nil {
		log.Println("DLR notification err:", err)
		return
	}
	req, err := http.NewRequest("POST", reqJson.DlrUrl, bytes.NewBuffer(dlrBytes))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
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
