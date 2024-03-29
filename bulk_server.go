package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"net/http/httputil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// text which simulate bulk bug - to panic
const panicMessage = "PANIC"

// text which simulate bulk bug - to response too late
const timeoutMessage = "TIMEOUT"

var errorCodes = map[string]string{
	"101": "Internal application error.",
	"102": "Encoding not supported or message not encoded with given encoding.",
	"103": "No account with given username/password.",
	"104": "Sending from clients IP address not allowed.",
	"105": "Too many messages submitted within short period of time. Resend later.",
	"106": "Sender contains words blacklisted on destination.",
	"107": "Sender contains illegal characters.",
	"108": "Message (not split automatically by HORISEN BULK Service, but by customer) is too long.",
	"109": "Format of text/content parameter is wrong.",
	"110": "Mandatory parameter is missing.",
	"111": "Unknown message type.",
	"112": "Format of some parameter is wrong.",
	"113": "No credit on account balance.",
	"114": "No route for given destination.",
	"115": "Message cannot be split into concatenated messages (e.g. too many parts will be needed).",
}

var dlrErrorCodes = map[string]string{
	"1":   "Unknown subscriber",
	"9":   "Illegal subscriber",
	"11":  "Teleservice not provisioned",
	"13":  "Call barred",
	"15":  "CUG reject",
	"19":  "No SMS support in MS",
	"20":  "Error in MS",
	"21":  "Facility not supported",
	"22":  "Memory capacity exceeded",
	"29":  "Absent subscriber",
	"30":  "MS busy for MT SMS",
	"36":  "Network/Protocol failure",
	"44":  "Illegal equipment",
	"60":  "No paging response",
	"61":  "GMSC congestion",
	"63":  "HLR timeout",
	"64":  "MSC/SGSN_timeout",
	"70":  "SMRSE/TCP error",
	"72":  "MT congestion",
	"75":  "GPRS suspended",
	"80":  "No paging response via MSC",
	"81":  "IMSI detached",
	"82":  "Roaming restriction",
	"83":  "Deregistered in HLR for GSM",
	"84":  "Purged for GSM",
	"85":  "No paging response via SGSN",
	"86":  "GPRS detached",
	"87":  "Deregistered in HLR for GPRS",
	"88":  "The MS purged for GPRS",
	"89":  "Unidentified subscriber via MSC",
	"90":  "Unidentified subscriber via SGSN",
	"112": "Originator missing credit on prepaid account",
	"113": "Destination missing credit on prepaid account",
	"114": "Error in prepaid system",
	"500": "Other error",
	"988": "MNP Error",
	"989": "Supplier rejected SMS",
	"990": "HLR failure",
	"991": "Rejected by message text filter",
	"992": "Ported numbers not supported on destination",
	"993": "Blacklisted sender",
	"994": "No credit",
	"995": "Undeliverable",
	"996": "Validity expired",
	"997": "Blacklisted receiver",
	"998": "No route",
	"999": "Repeated submission (possible looping)",
}

var matchErr = regexp.MustCompile("ERR-([0-9]+)")
var matchDlrErr = regexp.MustCompile("DLR-(DELIVERED|UNDELIVERED|BUFFERED|SENT_TO_SMSC|REJECTED)-([0-9]+)")
var matchDlrDelay = regexp.MustCompile("DLR-DELAYED-([0-9]+)")

// BulkRequestAuth is auth related embed struct
type BulkRequestAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// BulkRequest is new SMS request
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

// BulkResultSuccess is success response
type BulkResultSuccess struct {
	MsgID    string `json:"msgId"`
	NumParts int    `json:"numParts"`
}

// BulkError is error response
type BulkError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// BulkResultError is response error wrapper
type BulkResultError struct {
	Error BulkError `json:"error"`
}

// BulkDlr is dlr update
type BulkDlr struct {
	MsgID        string `json:"msgId"`
	Event        string `json:"event"`
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
	PartNum      int    `json:"partNum"`
	NumParts     int    `json:"numParts"`
	AccountName  string `json:"accountName"`
	delayed      int
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
func serveBulkServer(hub *Hub, httpClient *http.Client, w http.ResponseWriter, r *http.Request) {
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
	//try to match simulator error pattern
	errMatch := matchErr.FindStringSubmatch(reqJSON.Text)
	if errMatch != nil && len(errMatch) == 2 {
		log.Println("Bulk request matched pattern to simulate ERR response")
		jsonResult(w, 420, makeErrorResult(errMatch[1], errorCodes[errMatch[1]]))
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

	//check if obtained regular number of parts: 1 to 6
	if smsParts < 1 || smsParts > 6 {
		log.Println("The message is too long.")
		jsonResult(w, 420, makeErrorResult("108", "The message is too long."))
		return
	}

	//simulate long timeout
	if reqJSON.Text == timeoutMessage {
		time.Sleep(time.Second * 45)
	}

	//close http conn. and flush
	if reqJSON.Text != panicMessage {
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
		PartNum:      0,
		NumParts:     smsParts,
		AccountName:  reqJSON.Auth.Username,
		delayed:      2,
	}

	//try to match simulator dlr error pattern
	dlrErrMatch := matchDlrErr.FindStringSubmatch(reqJSON.Text)
	if dlrErrMatch != nil && len(dlrErrMatch) == 3 {
		log.Println("Bulk request matched pattern to simulate DLR response")
		notificationDlr.Event = dlrErrMatch[1]
		notificationDlr.ErrorCode, _ = strconv.Atoi(dlrErrMatch[2])
		notificationDlr.ErrorMessage = dlrErrorCodes[dlrErrMatch[2]]
	}

	//try to match simulator dlr error pattern
	dlrDelayedMatch := matchDlrDelay.FindStringSubmatch(reqJSON.Text)
	if dlrDelayedMatch != nil && len(dlrDelayedMatch) == 2 {
		log.Println("Bulk request matched pattern to simulate DLR response delay")
		notificationDlr.delayed, _ = strconv.Atoi(dlrDelayedMatch[1])
	}

	//send dlr as go routine
	go sendDlr(httpClient, reqJSON, notificationDlr)

	if reqJSON.Text == panicMessage {
		panic("Panic on receiver [" + reqJSON.Receiver + "]")
	}
}

// send dlr
func sendDlr(httpClient *http.Client, reqJSON BulkRequest, notificationDlr BulkDlr) {
	log.Println("Sending DLR notification to ", reqJSON.DlrURL)
	//give a timeout
	time.Sleep(time.Duration(notificationDlr.delayed) * time.Second)

	//send dlr for all parts
	for i := notificationDlr.PartNum; i < notificationDlr.NumParts; i++ {
		notificationDlr.PartNum = i
		sendDlrPart(httpClient, reqJSON, notificationDlr)
	}
}

// send dlr part
func sendDlrPart(httpClient *http.Client, reqJSON BulkRequest, notificationDlr BulkDlr) {
	log.Printf("\nSending DLR notification for part %d to %s\n", notificationDlr.PartNum, reqJSON.DlrURL)

	dlrBytes, err := json.Marshal(notificationDlr)
	if err != nil {
		log.Println("DLR notification err:", err)
		return
	}
	req, err := http.NewRequest("POST", reqJSON.DlrURL, bytes.NewBuffer(dlrBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
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
	rtext := []rune(text)
	gsm7bitChars := "\\@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞÆæßÉ !\"#¤%%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà^{}[~]|€"

	for _, c := range rtext {
		if !strings.ContainsRune(gsm7bitChars, c) {
			return false
		}
	}
	return true
}

func getNumberOfSMSsegments(text string, maxSegments int) int {
	totalSegment := 0
	rtext := []rune(text)
	textLen := len(rtext)
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
		totalSegment = int(math.Ceil(float64(textLen) / float64(concatMax)))
	}
	if totalSegment > maxSegments {
		return 0 //SMS is very big.
	}
	return totalSegment
}
