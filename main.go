package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ApiItem struct {
	Method string `json:"method"`
	Url    string `json:"url"`
}

var pendingMethod string
var pendingUrl string
var paramMatch = regexp.MustCompile(`\{(\w+)\}`)
var apiAdded ApiItem
var auth string
var body string

func extractUrlParam(url string) []string {
	matches := paramMatch.FindAllStringSubmatch(url, -1)
	var params []string
	for _, item := range matches {
		if len(item) >= 2 {
			params = append(params, item[1])
		}
	}
	return params
}

func prettyJSON(raw string) string {
	var data any
	err := json.Unmarshal([]byte(raw), &data)
	if err != nil {
		return raw
	}
	buf, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return raw
	}
	return string(buf)
}

func extractApiFromJSONFile(path string) []ApiItem {
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var list []ApiItem
	err = json.Unmarshal(content, &list)
	if err != nil {
		panic(err)
	}
	return list
}

func saveAPIToFile(path string, apis []ApiItem) {
	data, err := json.MarshalIndent(apis, "", " ")
	if err != nil {
		panic(err)
	}
	os.WriteFile(path, data, 0644)
}

func main() {
	apis := extractApiFromJSONFile("./apis.json")

	app := tview.NewApplication()

	newPrimitive := func(text string, color tcell.Color) tview.Primitive {
		return tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetText(text).
			SetTextColor(color)
	}

	footer := tview.NewTextView().SetTextAlign(tview.AlignCenter).
		SetText("[j/k] nav  [f] filter  [d] del  [Space] send  [Tab] Change Page")
	grid := tview.NewGrid().SetRows(3, 0, 1).SetColumns(28, 40, 0).SetBorders(true).SetBordersColor(tcell.ColorDimGray)
	list := tview.NewList().SetHighlightFullLine(true)
	updateList := func(filter string) {
		list.Clear()
		for _, api := range apis {
			if strings.Contains(strings.ToLower(api.Url), strings.ToLower(filter)) {
				list.AddItem(api.Method+" "+api.Url, "", 0, nil)
			}
		}
	}
	updateList("")

	apiBeRunning := tview.NewTextView().SetTextAlign(tview.AlignCenter).SetDynamicColors(true).
		SetTextColor(tcell.ColorWhite)
	filterInput := tview.NewInputField().
		SetLabel("[::d]filter[::-]:").
		SetLabelColor(tcell.ColorDimGray).
		SetFieldBackgroundColor(tcell.ColorWhite).
		SetFieldTextColor(tcell.ColorBlack).
		SetLabelWidth(8)
	response := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetScrollable(true).
		SetDynamicColors(true).
		SetTextColor(tcell.ColorWhite)
	bodyContent := tview.NewTextArea()
	paramContent := tview.NewTextArea()
	authContent := tview.NewTextArea()
	methodInput := tview.NewInputField().SetFieldTextColor(tcell.ColorBlack).SetLabel("Method:")
	urlInput := tview.NewInputField().SetFieldTextColor(tcell.ColorBlack).SetLabel("Url:")

	apiList := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive(" API LIST ", tcell.ColorGreen), 2, 0, false).
		AddItem(filterInput, 2, 0, false).
		AddItem(list, 0, 1, true)

	runBoard := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive(" RUNNING ", tcell.ColorBlue), 2, 0, false).
		AddItem(apiBeRunning, 0, 1, false)

	resArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive(" RESPONSE ", tcell.ColorYellow), 2, 0, false).
		AddItem(response, 0, 1, false)

	paramArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive(" PARAM ", tcell.ColorDarkCyan), 2, 0, false).
		AddItem(paramContent, 0, 1, false)

	bodyArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive(" BODY ", tcell.ColorDarkMagenta), 2, 0, false).
		AddItem(bodyContent, 0, 1, false)

	authArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive(" AUTH ", tcell.ColorDarkCyan), 2, 0, false).
		AddItem(authContent, 0, 1, false)

	listAddFrom := tview.NewForm().AddFormItem(methodInput).AddFormItem(urlInput)
	listAddArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive(" API ADD ", tcell.ColorBlue), 2, 0, false).
		AddItem(listAddFrom, 0, 1, false)

	page := tview.NewPages().
		AddPage("Param", paramArea, true, true).
		AddPage("Body", bodyArea, true, false).
		AddPage("Auth", authArea, true, false).
		AddPage("Add", listAddArea, true, false)

	pageChangeList := []string{"Param", "Body", "Auth", "Add"}
	pageChangeBox := map[string]tview.Primitive{
		"Param": paramContent,
		"Body":  bodyContent,
		"Auth":  authContent,
		"Add":   listAddFrom,
	}

	grid.AddItem(apiList, 0, 0, 2, 1, 0, 0, true).
		AddItem(runBoard, 0, 1, 1, 2, 0, 0, false).
		AddItem(resArea, 1, 2, 1, 1, 0, 0, false).
		AddItem(page, 1, 1, 1, 1, 0, 0, false).
		AddItem(footer, 2, 0, 1, 3, 0, 0, false)

	sendRequest := func(method string, url string, body string, auth string) {
		go func() {
			var reqBody io.Reader
			if body != "" {
				reqBody = strings.NewReader(body)
			}
			req, _ := http.NewRequest(method, "http://"+url, reqBody)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(auth))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				app.QueueUpdateDraw(func() {
					response.SetText("bad request:" + err.Error())
				})
				return
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				app.QueueUpdateDraw(func() {
					response.SetText("Error:" + err.Error())
				})
				return
			}
			app.QueueUpdateDraw(func() {
				response.SetText(prettyJSON(string(data)))
			})
		}()
	}

	filterInput.SetChangedFunc(func(text string) {
		updateList(text)
	})

	filterInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			updateList(filterInput.GetText())
			app.SetFocus(list)
		}
		if key == tcell.KeyESC {
			filterInput.SetText("")
			app.SetFocus(list)
		}
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		current := list.GetCurrentItem()
		total := list.GetItemCount()

		switch {

		case event.Key() == tcell.KeyTab:
			name, _ := page.GetFrontPage()
			app.SetFocus(pageChangeBox[name])
			return nil
		case event.Key() == tcell.KeyEnter:
			pendingMethod = apis[current].Method
			pendingUrl = apis[current].Url
			apiBeRunning.SetText(pendingMethod + ":" + pendingUrl + " is waitting ...")
			paramMap := extractUrlParam(pendingUrl)
			var initText string
			for _, param := range paramMap {
				initText += fmt.Sprintf("%v:\n", param)
			}
			paramContent.SetText(initText, false)
		case event.Rune() == 'j':
			if current < total-1 {
				list.SetCurrentItem(current + 1)
			}
			return nil
		case event.Rune() == 'k':
			if current > 0 {
				list.SetCurrentItem(current - 1)
			}
			return nil
		case event.Rune() == 'f':
			app.SetFocus(filterInput)
			return nil
		case event.Rune() == 'd':
			apis = append(apis[:current], apis[current+1:]...)
			updateList("")
			return nil
		}

		return event
	})

	current := 0
	page.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlA {
			current = (current + 1) % len(pageChangeList)
			page.SwitchToPage(pageChangeList[current])
			app.SetFocus(pageChangeBox[pageChangeList[current]])
			return nil
		}

		if event.Key() == tcell.KeyTab {
			app.SetFocus(response)
		}
		return event
	})

	listAddFrom.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlS {
			apiAdded.Method = methodInput.GetText()
			apiAdded.Url = urlInput.GetText()
			apis = append(apis, apiAdded)
			list.AddItem(apiAdded.Method+" "+apiAdded.Url, "", 0, nil)
			updateList("")
			saveAPIToFile("./apis.json", apis)
			return nil
		}
		return event
	})

	response.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			app.SetFocus(list)
			return nil
		}
		return event
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == ' ' {
			textInput := paramContent.GetText()
			splitString := strings.Split(textInput, "\n")
			var key string
			var value string
			for _, str := range splitString {
				str = strings.TrimSpace(str)
				if str == "" {
					continue
				}
				keyAndValue := strings.SplitN(str, ":", 2)
				if len(keyAndValue) < 2 {
					continue
				}
				key = keyAndValue[0]
				value = keyAndValue[1]
				pendingUrl = strings.ReplaceAll(pendingUrl, "{"+key+"}", value)
			}
			body = bodyContent.GetText()
			auth = authContent.GetText()
			sendRequest(pendingMethod, pendingUrl, body, auth)
			apiBeRunning.SetText(pendingMethod + ":" + pendingUrl + " is sent")

			return nil
		}
		if event.Key() == tcell.KeyEsc {
			app.SetFocus(list)
		}
		return event
	})

	if err := app.SetRoot(grid, true).Run(); err != nil {
		panic(err)
	}
}
