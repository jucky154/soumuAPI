package main

import (
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"github.com/tadvi/winc"
	"io/ioutil"
	"net/http"
	"strings"
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

func accessAPI() (*SearchResult, error) {
	//空データを作る
	data := new(SearchResult)
	//コールサインをzlogから取得
	callSign := Query("$B")
	if len(callSign) < 4 {
		err := errors.New("callsign too short")
		DisplayToast(err.Error())
		return data, err
	}

	// APIからjsonデータを取得
	url := "https://www.tele.soumu.go.jp/musen/list?ST=1&OF=2&DA=1&OW=AT&SK=2&DC=1&SC=1&MA=" + callSign
	resp, err := http.Get(url)
	defer resp.Body.Close()

	//httpアクセスでエラーを吐いた時は出る
	if err != nil {
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

func update(data SearchResult, frequency string) {
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
			Number:   number,
			Power:    power,
		})
	}
}

func freqstring(index int) string {
	switch {
	case index == 0:
		return "1910"
	case index == 1:
		return "3537.5"
	case index == 2:
		return "7100"
	case index == 3:
		return "10125"
	case index == 4:
		return "14175"
	case index == 5:
		return "18118"
	case index == 6:
		return "21225"
	case index == 7:
		return "24940"
	case index == 8:
		return "28.85"
	case index == 9:
		return "52"
	case index == 10:
		return "145"
	case index == 11:
		return "435"
	case index == 12:
		return "1280"
	case index == 13:
		return "2425"
	case index == 14:
		return "5750"
	default:
		return "1910"
	}
}

func makewindow() {
	// --- Make Window
	mainWindow := winc.NewForm(nil)
	mainWindow.SetSize(800, 600)
	mainWindow.SetText("soumuAPI")

	combo := winc.NewComboBox(mainWindow)
	combo.InsertItem(0, "1.9MHz")
	combo.InsertItem(1, "3.5MHz")
	combo.InsertItem(2, "7MHz")
	combo.InsertItem(3, "10MHz")
	combo.InsertItem(4, "14MHz")
	combo.InsertItem(5, "18MHz")
	combo.InsertItem(6, "21MHz")
	combo.InsertItem(7, "24MHz")
	combo.InsertItem(8, "28MHz")
	combo.InsertItem(9, "50MHz")
	combo.InsertItem(10, "144MHz")
	combo.InsertItem(11, "430MHz")
	combo.InsertItem(12, "1.2GHz")
	combo.InsertItem(13, "2.4GHz")
	combo.InsertItem(14, "5.6GHz")
	combo.SetSelectedItem(0)

	btn := winc.NewPushButton(mainWindow)
	btn.SetText("check")
	btn.SetPos(40, 50)
	btn.SetSize(100, 40)

	btn.OnClick().Bind(func(e *winc.Event) {
		index := combo.SelectedItem()
		data, err := accessAPI()
		freq := freqstring(index)
		if err == nil {
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
	dock.Dock(combo, winc.Top)
	dock.Dock(btn, winc.Top)

	mainWindow.Show()
}

func init() {
	OnLaunchEvent = onLaunchEvent
}

func onLaunchEvent() {
	readACAG()
	makewindow()
}
