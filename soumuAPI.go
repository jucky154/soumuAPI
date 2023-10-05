package main

import (
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"github.com/sqweek/dialog"
	"github.com/tadvi/winc"
	"io/ioutil"
	"net/http"
	"strings"
	"zylo/reiwa"
	"zylo/win32"
)

const winsize = "soumuAPIwindow"

// datファイルを読み込み
//
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
	Number   string
	Power    string
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

var numberTable [][]string

// 市町村名　ナンバーのリストを整理する
func readACAG() {
	// 形式: [市郡町村, ナンバー] のリスト
	numberReader := csv.NewReader(strings.NewReader(acagList))
	numberReader.Comma = '\t'
	numberTable, _ = numberReader.ReadAll()
}

func accessAPI() (*SearchResult, error) {
	//空データを作る
	data := new(SearchResult)
	//コールサインをzlogから取得
	callSign := reiwa.Query("$B")
	if len(callSign) < 4 {
		err := errors.New("callsign too short")
		reiwa.DisplayToast(err.Error())
		return data, err
	}

	// APIからjsonデータを取得
	url := "https://www.tele.soumu.go.jp/musen/list?ST=1&OF=2&DA=1&OW=AT&SK=2&DC=1&SC=1&MA=" + callSign
	resp, err := http.Get(url)
	defer resp.Body.Close()

	//httpアクセスでエラーを吐いた時は出る
	if err != nil {
		reiwa.DisplayToast(err.Error())
		return data, err
	}

	byteArray, _ := ioutil.ReadAll(resp.Body)
	jsonBytes := ([]byte)(byteArray)

	// unmarshalに操作失敗したらエラー
	if err := json.Unmarshal(jsonBytes, data); err != nil {
		reiwa.DisplayToast(err.Error())
		return data, err
	}
	return data, nil
}

func update(data SearchResult) {
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
		power := freqstring(strings.TrimSpace(info.RadioSpec1))

		stationview.list.AddItem(StationItem{
			CallSign: callSign,
			Location: location,
			Number:   number,
			Power:    power,
		})
	}
}

func freqstring(index string) string {
	switch {
	case index == "1AF":
		return "1アマ固定"
	case index == "1AM":
		return "1アマ移動"
	case index == "2AF":
		return "2アマ固定"
	case index == "2AM":
		return "2アマ移動"
	case index == "3AF":
		return "3アマ固定"
	case index == "3AM":
		return "3アマ移動"
	case index == "4AF":
		return "4アマ固定"
	case index == "4AM":
		return "4アマ移動"
	default:
		return "不明"
	}
}

func btnpush() {
	data, err := accessAPI()
	if err == nil {
		update(*data)
	}
	return
}

var mainWindow *winc.Form

func makewindow() {
	// --- Make Window
	mainWindow = win32.NewForm(nil)

	btn := winc.NewPushButton(mainWindow)
	btn.SetText("check")
	btn.SetPos(40, 50)
	btn.SetSize(100, 40)

	btn.OnClick().Bind(func(e *winc.Event) {
		go btnpush()
	})

	stationview.list = winc.NewListView(mainWindow)
	stationview.list.EnableEditLabels(false)
	stationview.list.AddColumn("callsign", 120)
	stationview.list.AddColumn("location", 200)
	stationview.list.AddColumn("number", 120)
	stationview.list.AddColumn("license", 120)
	dock := winc.NewSimpleDock(mainWindow)
	dock.Dock(stationview.list, winc.Fill)
	dock.Dock(btn, winc.Top)

	mainWindow.Show()
}

func init() {
	reiwa.OnLaunchEvent = onLaunchEvent
	reiwa.PluginName = "soumuAPI"
}

func onLaunchEvent() {
	reiwa.RunDelphi(`PluginMenu.Add(op.Put(MainMenu.CreateMenuItem(), "Name", "PluginsoumuAPIWindow"))`)
	reiwa.RunDelphi(`op.Put(MainMenu.FindComponent("PluginsoumuAPIWindow"), "Caption", "総務省API ウィンドウ")`)

	reiwa.RunDelphi(`PluginMenu.Add(op.Put(MainMenu.CreateMenuItem(), "Name", "PluginsoumuAPIRule"))`)
	reiwa.RunDelphi(`op.Put(MainMenu.FindComponent("PluginsoumuAPIRule"), "Caption", "総務省API 利用規約")`)

	reiwa.HandleButton("MainForm.MainMenu.PluginsoumuAPIWindow", func(num int) {
		readACAG()
		makewindow()
	})

	reiwa.HandleButton("MainForm.MainMenu.PluginsoumuAPIRule", func(num int) {
		dialog.Message("%s", "このサービスは、総務省 電波利用ホームページのWeb-API 機能を利用して取得した情報をもとに作成していますが、サービスの内容は総務省によって保証されたものではありません").Title("利用規約").Info()
	})
}
