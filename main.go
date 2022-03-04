package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"

	"github.com/yokoe/herschel"
	"github.com/yokoe/herschel/option"
)

// アクセストークン、URL を定義
const ACCESSTOKEN = "xxxxxxxxx"
const ENDPOINT = "http://192.168.3.22/api/v4/posts"
const ID = "xxxxxxxxxx"
const sheet = "シート1"
const CHANNELID = "xxxxxxxx"

var EVENTLIST [][]string
var auth = "xxxxxxxxxxx"
var Meetings int

type BODY struct {
	ChannelId string `json:"channel_id"`
	Message   string `json:"message"`
}

func ExistFile(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func HTTPPost(mes string) {

	//debug用
	debug := strings.Split(mes, "_")[0]

	//BODYの作成
	requestBody := new(BODY)
	requestBody.ChannelId = CHANNELID
	requestBody.Message = mes

	// ボディをJSONに変換
	json_body, _ := json.Marshal(requestBody)

	//リクエストを作成する
	req, _ := http.NewRequest("POST", ENDPOINT, bytes.NewBuffer(json_body))
	req.Header.Set("Authorization", ACCESSTOKEN)
	req.Header.Set("Content-Type", "application/json")

	//リクエストを送信
	client := new(http.Client)
	resp, err := client.Do(req)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	//デバッグ用 POSTの内容を確認
	if debug == "debug" {
		//リクエストの内容確認
		dump, _ := httputil.DumpRequestOut(req, true)
		fmt.Printf("%s", dump)

		//POST の内容を確認
		dumpResp, _ := httputil.DumpResponse(resp, true)
		fmt.Printf("%s", dumpResp)
	}
}

// 参加している人のメールアドレスを取得
func TakeMailAddress(MeetingNumber int) []string {

	meetingnumber := strconv.Itoa(MeetingNumber)
	req, err := http.NewRequest("GET", "https://webexapis.com/v1/meetings?meetingNumber="+meetingnumber, nil)
	if err != nil {
		fmt.Println(err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+auth)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var bodyMap map[string]interface{}
	json.Unmarshal(body, &bodyMap)
	meetingID := bodyMap["items"].([]interface{})[0].(map[string]interface{})["id"].(string)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	req2, err2 := http.NewRequest("GET", "https://webexapis.com/v1/meetingParticipants?meetingId="+meetingID, nil)
	if err2 != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	req2.Header.Add("Content-Type", "application/json")
	req2.Header.Add("Authorization", "Bearer "+auth)

	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		fmt.Println(err)
	}

	body2, err := ioutil.ReadAll(res2.Body)
	if err != nil {
		fmt.Println(err)
	}

	json.Unmarshal(body2, &bodyMap)

	var addresslist []string
	for i := 0; i < len(bodyMap["items"].([]interface{})); i++ {
		addresslist = append(addresslist, bodyMap["items"].([]interface{})[i].(map[string]interface{})["email"].(string))
	}

	return addresslist
}

// スプレットシート上でメールアドレスと一致する列を返す関数
func MailToRow(mailaddress string) int {

	//スプレットシート上でメールアドレスを探索
	// サービスアカウントの認証情報を使って、スプレッドシートにアクセスするためのClientを用意
	client, err := herschel.NewClient(option.WithServiceAccountCredentials("credentials.json"))
	if err != nil {
		HTTPPost("Error: スプレットシートにアクセスできませんでした。")
		os.Exit(1)
	}

	//テーブルを取得
	table, err := client.ReadTable(ID, sheet)
	if err != nil {
		fmt.Println(err)
	}

	count := 0
	row := 0
	for {
		if table.GetStringValue(0, count) == mailaddress {
			row = count
			break
		}
		if count > 100 {
			HTTPPost("Error: ユーザーが見つかりませんでした。")
			row = -1
			break
		}
		count++
	}

	//みつけた列を返却
	return row
}

// (x,y) に message を書く
func writeXY(x int, y int, message string) {
	// スプレットシートにアクセス
	client, err := herschel.NewClient(option.WithServiceAccountCredentials("credentials.json"))
	if err != nil {
		HTTPPost("Error: スプレットシートにアクセスできませんでした。")
		os.Exit(1)
	}

	table := herschel.NewTable(50, 50)
	table.PutValue(y-1, x-1, message)

	err = client.WriteTable(ID, sheet, table)
	if err != nil {
		fmt.Println(err)
	}
}

// n行目のメールアドレスの人の出欠をとる
func attend(mailaddress []string, n int) {

	for i := 0; i < len(mailaddress); i++ {
		//MailToRowを実行してそのメールアドレスの列数rowを取得
		row := MailToRow(mailaddress[i])

		if row < 0 {
			HTTPPost("Error: 列数が異常です。")
			break
		} else {
			writeXY(row+1, n, "1")
		}
	}
}

func attending(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.FormValue("text"))

	if Meetings == 0 {
		HTTPPost("meetingIDが指定されていません。")
	} else {
		maillist := TakeMailAddress(Meetings)
		attend(maillist, n)
	}
}

// /set : meetingID をセットする関数
func set(w http.ResponseWriter, r *http.Request) {
	num, _ := strconv.Atoi(r.FormValue("text"))
	Meetings = num
	HTTPPost("meetingID を " + strconv.Itoa(num) + " に変更します。")
}

// /init : 初期化、スプレットシートをきれいにする
func initting(w http.ResponseWriter, r *http.Request) {
	InitFunc()
}

func InitFunc() {
	HTTPPost("スプレットシートを初期化します。")

	// サービスアカウントの認証情報を使って、スプレッドシートにアクセスするためのClientを用意
	client, err := herschel.NewClient(option.WithServiceAccountCredentials("credentials.json"))
	if err != nil {
		HTTPPost("Error: スプレットシートにアクセスできませんでした。")
		os.Exit(1)
	}

	//日付を描く
	put := herschel.NewTable(100, 100)

	//名前を書く
	f, err := os.Open("name.csv")
	if err != nil {
		fmt.Println(err)
	}
	r := csv.NewReader(f)

	var name []string
	var address []string

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
		}
		address = append(address, record[0])
		name = append(name, record[1])
	}

	put = herschel.NewTable(100, 100)
	put.PutValue(0, 0, "月日")
	put.PutValue(0, 1, "内容")

	for index := 0; index < len(name); index++ {
		put.PutValue(0, index+2, address[index])
		put.PutValue(1, index+2, name[index])
	}

	err = client.WriteTable(ID, sheet, put)

	//put.SetBackgroundColor(datenum,2,color.RGBA{255, 255, 200, 255})
	err = client.WriteTable(ID, sheet, put)
}

func main() {

	HTTPPost("tetsuya bot 起動")

	//name.csv、credentials.json ファイルが存在するかどうかチェック
	if !(ExistFile("name.csv") && ExistFile("credentials.json")) {
		HTTPPost("認証ファイルが存在しません。")
		os.Exit(1)
	}

	http.HandleFunc("/attend", attending)
	http.HandleFunc("/set", set)
	http.HandleFunc("/init", initting)

	err := http.ListenAndServe(":8111", nil)
	if err != nil {
		fmt.Println(err)
	}

	HTTPPost("tetsuya bot 終了")
}
