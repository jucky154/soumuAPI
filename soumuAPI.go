package main

import (
	"encoding/csv"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"errors"
	_ "embed"
	"github.com/tadvi/winc"
	"github.com/sqweek/dialog"
	"unsafe"
	"github.com/gonutz/w32"
)

// datファイルを読み込み
//go:embed allcity.dat
var acagList string

type SearchResult struct {
	Musen []struct {
		DetailInfo struct {
			RadioSpec1            string `json:"radioSpec1"`
			IdentificationSignals string `json:"identificationSignals"`
			RadioEuipmentLocation string `json:"radioEuipmentLocation"`
		} `json:"detailInfo"`
	} `json:"musen"`
}

type StationView struct {
	list *winc.ListView
}

var stationview StationView

type StationItem struct {
	CallSign string
	Location string
	Number string
	Power string
}

func (item StationItem) Text() (text []string) {
	text = append(text, item.CallSign)
	text = append(text, item.Location)
	text = append(text, item.Number)
	text = append(text, item.Power)
	return
}

func (item StationItem) ImageIndex() int {
	return 0
}

var numberTable  [][]string

// RadioSpec1からテーブルを生成
// 形式: [型式, 周波数, 空中線電力] のリスト
func spec1StringToArray(spec string) [][]string {
	specFormatted := strings.ReplaceAll(strings.ReplaceAll(spec, `\t`, "\t"), `\n`, "\n")

	specReader := csv.NewReader(strings.NewReader(specFormatted))
	specReader.Comma = '\t'
	specTable, _ := specReader.ReadAll()
	return specTable
}

// 市町村名　ナンバーのリストを整理する
func readACAG() {
	// 形式: [市郡町村, ナンバー] のリスト
	numberReader := csv.NewReader(strings.NewReader(acagList))
	numberReader.Comma = '\t'
	numberTable, _ = numberReader.ReadAll()
}


func accessAPI() (*SearchResult, error){
	//空データを作る
	data := new(SearchResult)
	//コールサインをzlogから取得
	callSign := Query("$B")
	if len(callSign) < 4{
		err := errors.New("callsign too short")
		DisplayToast(err.Error())
		return data, err
	}

	// APIからjsonデータを取得
	url := "https://www.tele.soumu.go.jp/musen/list?ST=1&OF=2&DA=1&OW=AT&SK=2&DC=1&SC=1&MA=" + callSign
	resp, err:= http.Get(url)
	defer resp.Body.Close()

	//httpアクセスでエラーを吐いた時は出る
	if  err != nil {
		DisplayToast(err.Error())
		return data, err
	}

	byteArray, _ := ioutil.ReadAll(resp.Body)
	jsonBytes := ([]byte)(byteArray)

	// unmarshalに操作失敗したらエラー
	if err := json.Unmarshal(jsonBytes, data); err != nil {
		DisplayToast(err.Error())
		return data, err
	}
	return data, nil
}

func update(data SearchResult, frequency string){
	//listを消す
	stationview.list.DeleteAllItems()
	// 検索にヒットした局ごとにコールサイン、JCC/JCGナンバーを出力
	for _, radioStation := range data.Musen {
		info := radioStation.DetailInfo
		callSign := strings.TrimSpace(info.IdentificationSignals)
		// datファイルを全探索してコンテストナンバーを検索
		// 速度改善の余地あり
		location := info.RadioEuipmentLocation
		number := "ナンバー不明"
		for _, row := range numberTable {
			if row[0] == location {
				number = row[1]
			}
		}

		// info.RadioSpec1より周波数帯の出力
		power := "発射不可"
		for _, row := range spec1StringToArray(info.RadioSpec1) {
			if strings.Contains(row[1], frequency) {
				power = strings.ReplaceAll(row[2], " ", "")
			}
		}
		stationview.list.AddItem(StationItem{
			CallSign: callSign,
			Location: location,
			Number: number,
			Power: power,
		})
	}
}

func freqstring(index string) string{
	switch{
		case index == "1.9" : 
			return "1910"
		case index == "3.5" : 
			return "3537.5"
		case index == "7" : 
			return "7100"
		case index == "10" : 
			return "10125"
		case index == "14" : 
			return "14175"
		case index == "18" : 
			return "18118"
		case index == "21" : 
			return "21225"
		case index ==  "24": 
			return "24940"
		case index == "28" : 
			return "28.85"
		case index == "50" : 
			return "52"
		case index == "144" : 
			return "145"
		case index == "430" : 
			return "435"
		case index == "1200" : 
			return "1280"
		case index == "2400" : 
			return "2425"
		case index == "5600" : 
			return "5750"
		case index == "10G" : 
			return "10.125"
		default:
			return "1910"
	}
}
	
var mainWindow *winc.Form

func wndOnClose(arg *winc.Event){
	mainWindow.Close()
}

func makewindow(){
	// --- Make Window
	mainWindow = winc.NewForm(nil)
	mainWindow.SetSize(800, 600)
	mainWindow.SetText("soumuAPI")

	btn := winc.NewPushButton(mainWindow)
	btn.SetText("check")
	btn.SetPos(40,50)
	btn.SetSize(100,40)

	btn.OnClick().Bind(func(e *winc.Event){
		data, err := accessAPI()
		freq := freqstring(Query("{B}"))
		if err == nil{
			update(*data, freq)
		}
	})

	stationview.list = winc.NewListView(mainWindow)
	stationview.list.EnableEditLabels(false)
	stationview.list.AddColumn("callsign", 120)
	stationview.list.AddColumn("location", 120)
	stationview.list.AddColumn("number", 120)
	stationview.list.AddColumn("power", 120)
	dock := winc.NewSimpleDock(mainWindow)
	dock.Dock(stationview.list, winc.Fill)
	dock.Dock(btn, winc.Top)

	mainWindow.Show()
	mainWindow.OnClose().Bind(wndOnClose)
}

func init() {
	OnLaunchEvent = onLaunchEvent
	OnWindowEvent = onWindowEvent
}


func onLaunchEvent() {
	hMenu1 := w32.HMENU(GetUI("MainForm.MainMenu"))
	hMenu2 := w32.CreateMenu()
	w32.AppendMenu(hMenu1, w32.MF_POPUP, uintptr(hMenu2), "総務省API")
	w32.AppendMenu(hMenu2, w32.MF_STRING, 10001, "ウィンドウを開く")
	w32.AppendMenu(hMenu2, w32.MF_STRING, 10002, "利用規約")
	w32.DrawMenuBar(w32.HWND(GetUI("MainForm")))
}

func onWindowEvent(ptr uintptr) {
	msg := (*w32.MSG)(unsafe.Pointer(ptr))
	if msg.Message == w32.WM_COMMAND {
		switch msg.WParam {
 		case 10001:
			readACAG()
			makewindow()
		case 10002:
			dialog.Message("%s", "このサービスは、総務省 電波利用ホームページのWeb-API 機能を利用して取得した情報をもとに作成していますが、サービスの内容は総務省によって保証されたものではありません").Title("利用規約").Info()
		}
	}
}
