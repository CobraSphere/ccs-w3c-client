/**
* (C) 2020 Geotab Inc
*
* All files and artifacts in the repository at https://github.com/UlfBj/ccs-w3c-client
* are licensed under the provisions of the license provided by the LICENSE file in this repository.
*
**/

package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"flag"
	"os"
	"strconv"
	"strings"

	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

var vissv2Url string
var ovdsUrl string
var thisVin string

type PathList struct {
	LeafPaths []string
}

var pathList PathList

func pathToUrl(path string) string {
	var url string = strings.Replace(path, ".", "/", -1)
	return "/" + url
}

func jsonToStructList(jsonList string) int {
	err := json.Unmarshal([]byte(jsonList), &pathList)
	if err != nil {
		fmt.Printf("Error unmarshal json=%s\n", err)
		pathList.LeafPaths = nil
		return 0
	}
	return len(pathList.LeafPaths)
}

func createListFromFile(fname string) int {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return 0
	}
	return jsonToStructList(string(data))
}

func saveListAsFile(fname string) {
	buf, err := json.Marshal(pathList)
	if err != nil {
		fmt.Printf("Error marshalling from file %s: %s\n", fname, err)
		return
	}

	err = ioutil.WriteFile(fname, buf, 0644)
	if err != nil {
		fmt.Printf("Error writing file %s: %s\n", fname, err)
		return
	}
}

func getGen2Response(path string) string {
	url := "http://" + vissv2Url + ":8888" + pathToUrl(path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("getGen2Response: Error creating request=%s.", err)
		return ""
	}

	// Set headers
	req.Header.Set("Access-Control-Allow-Origin", "*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", vissv2Url+":8888")

	// Set client timeout
	client := &http.Client{Timeout: time.Second * 10}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("getGen2Response: Error in issuing request/response= %s ", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("getGen2Response: Error in reading response= %s ", err)
		return ""
	}

	return string(body)
}

func writeToOVDS(message string) {
	url := "http://" + ovdsUrl + ":8765/ovdsserver"
	fmt.Printf("writeToOVDS: message = %s\n", message)

	path, value, timeStamp := extractMessage(message)
	payload := `{"action":"set", "vin":"` + thisVin + `", "path":"` +  path + `", "value":"` + value + `", "timestamp":"` + timeStamp + `"}`
//	payload := `{"action":"set", "vin":"` + thisVin + `", ` +  data[1:]
	fmt.Printf("writeToOVDS: request payload= %s \n", payload)

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		fmt.Printf("writeToOVDS: Error creating request= %s \n", err)
		return
	}

	// Set headers
	req.Header.Set("Access-Control-Allow-Origin", "*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", ovdsUrl+":8765")

	// Set client timeout
	client := &http.Client{Timeout: time.Second * 10}

	// Send request
	_, err = client.Do(req)
	if err != nil {
		fmt.Printf("writeToOVDS: Error in issuing request/response= %s ", err)
		return
	}
	//	defer resp.Body.Close()

	/*	body, err := ioutil.ReadAll(resp.Body)   // TODO Handle error response
		if err != nil {
			fmt.Printf("writeToOVDS: Error in reading response= %s ", err)
			return
		}*/
}

func iterateGetAndWrite(elements int, sleepTime int) {
	for i := 0; i < elements; i++ {
		response := getGen2Response(pathList.LeafPaths[i])
		if (len(response) == 0) {
		    fmt.Printf("iterateGetAndWrite: Cannot connect to server.\n")
		    os.Exit(-1)
		}
		writeToOVDS(response)
	        time.Sleep((time.Duration)(sleepTime) * time.Millisecond)
	}
	fmt.Printf("\n\n****************** Iteration cycle over all paths completed ************************************\n\n")
}

func initVissV2WebSocket() *websocket.Conn {
	var addr = flag.String("addr", vissv2Url + ":8080", "http service address")
	dataSessionUrl := url.URL{Scheme: "ws", Host: *addr, Path: ""}
	conn, _, err := websocket.DefaultDialer.Dial(dataSessionUrl.String(), nil)
	if err != nil {
		fmt.Printf("Data session dial error:%s\n", err)
		os.Exit(-1)
	}
	return conn
}

func iterateNotificationAndWrite(conn *websocket.Conn) {
    for {
	_, msg, err := conn.ReadMessage()
	if err != nil {
	    fmt.Printf("Subscription response error: %s\n", err)
	    return
	}
	message := string(msg)
	if (strings.Contains(message, "subscribe") == true) {
	    fmt.Printf("Subscription response:%s\n", message)
	} else {
//	    var msgMap = make(map[string]interface{})
//	    jsonToMap(message, &msgMap)
//	    data, _ := json.Marshal(msgMap["data"])
//	    writeToOVDS(`{"data":` + string(data) + "}")
	    writeToOVDS(message)
	}
    }
}

func extractMessage(message string) (string, string, string) { // message is expected to contain the key-value: “data”:{“path”:”B”, “dp”:{“value”:”C”, “ts”:”D”}}
    var msgMap = make(map[string]interface{})
    jsonToMap(message, &msgMap)
    if (msgMap["data"] == nil) {
	fmt.Printf("Error: Message does not contain vehicle data.\n")
        return "", "", ""
    }
    data, _ := json.Marshal(msgMap["data"])

    jsonToMap(string(data), &msgMap)
    path := msgMap["path"].(string)
    dp, _ := json.Marshal(msgMap["dp"])

    jsonToMap(string(dp), &msgMap)
    value := msgMap["value"].(string)
    ts := msgMap["ts"].(string)
fmt.Printf("path=%s, value=%s, ts=%s\n", path, value, ts)
    return path, value, ts
}

func jsonToMap(request string, rMap *map[string]interface{}) {
	decoder := json.NewDecoder(strings.NewReader(request))
	err := decoder.Decode(rMap)
	if err != nil {
		fmt.Printf("jsonToMap: JSON decode failed for request:%s, err=%s\n", request, err)
	}
}

func subscribeToPaths(conn *websocket.Conn, elements int, sleepTime int) {
	for i := 0; i < elements; i++ {
		subscribeToPath(conn, pathList.LeafPaths[i])
	        time.Sleep((time.Duration)(sleepTime) * time.Millisecond)
	}
}

func subscribeToPath(conn *websocket.Conn, path string) {
    request := `{"action":"subscribe", "path":"` + path + `", "filter":{"op-type":"capture", "op-value":"time-based", "op-extra":{"period":"3"}}, "requestId": "6578"}`

    err := conn.WriteMessage(websocket.TextMessage, []byte(request))
    if err != nil {
	fmt.Printf("Subscribe request error:%s\n", err)
    }

}

func transferData(elements int, sleepTime int, accessMode string) {
	if (accessMode == "get") {
	    for {
		iterateGetAndWrite(elements, sleepTime)
	    }
	} else {
	    conn := initVissV2WebSocket()
	    go iterateNotificationAndWrite(conn)
	    subscribeToPaths(conn, elements, sleepTime)
	    for {
	        time.Sleep(1000 * time.Millisecond)  // just to keep alive...
	    }
	}
}

func main() {

	if len(os.Args) != 6 {
		fmt.Printf("CCS client command line: ./client gen2-server-url OVDS-server-url vin list-iteration-time access-mode\n")
		os.Exit(1)
	}
	vissv2Url = os.Args[1]
	ovdsUrl = os.Args[2]
	thisVin = os.Args[3]
	iterationPeriod, _ := strconv.Atoi(os.Args[4])
	accessMode := os.Args[5]
	if (accessMode != "get" && accessMode != "subscribe") {
		fmt.Printf("CCS client access-mode must be either get or subscribe.\n")
		os.Exit(1)
	}

	if createListFromFile("vsspathlist.json") == 0 {
	    if createListFromFile("../vsspathlist.json") == 0 {
		fmt.Printf("Failed in creating list from vsspathlist.json\n")
		os.Exit(1)
	    }
	}

	elements := len(pathList.LeafPaths)
	sleepTime := (iterationPeriod*1000-elements*30)/elements  // 30 = estimated time in msec for one roundtrip - get data from VISSv2 server, write data to OVDS
	if (sleepTime < 1) {
	    sleepTime = 1
	}
	transferData(elements, sleepTime, accessMode)
}
