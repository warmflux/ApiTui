package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ApiItem struct {
	Method    string
	Url       string
	NeedBody  bool
	NeedParam bool
}

var pendingMethod string
var pendingUrl string
var waitingParam bool = false
var waitingBody bool = false
var paramMatch = regexp.MustCompile(`\{(\w+)\}`)
var apiAdded ApiItem

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

func main() {
	app := tview.NewApplication()

	newPrimitive := func(text string, color tcell.Color) tview.Primitive {
		return tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetText(text).
			SetTextColor(color)
	}
	footer := tview.NewTextView().SetTextAlign(tview.AlignCenter).
		SetText("[j/k] nav  [f] filter  [a] add  [d] del  [Enter] send  [ESC] back  [Ctrl+S] confirm")
	grid := tview.NewGrid().SetRows(3, 0, 10, 1).SetColumns(28, 0).SetBorders(true).SetBordersColor(tcell.ColorDimGray)
	apis := []ApiItem{
		{
			Method:    "GET",
			Url:       "localhost:8081/api/users/{id}",
			NeedBody:  false,
			NeedParam: true,
		},
		{

			Method:    "PUT",
			Url:       "localhost:8081/api/users/{id}/email",
			NeedBody:  true,
			NeedParam: true,
		},
	}

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
	response := tview.NewTextView().SetTextAlign(tview.AlignLeft).SetScrollable(true).
		SetDynamicColors(true).SetTextColor(tcell.ColorWhite)
	bodyContent := tview.NewTextArea()
	paramContent := tview.NewTextArea()
	methodInput := tview.NewInputField().SetFieldBackgroundColor(tcell.ColorWhite).SetFieldTextColor(tcell.ColorBlack).SetLabel("Method:")
	urlInput := tview.NewInputField().SetFieldBackgroundColor(tcell.ColorWhite).SetFieldTextColor(tcell.ColorBlack).SetLabel("Url:")
	needBodyInput := tview.NewInputField().SetFieldBackgroundColor(tcell.ColorWhite).SetFieldTextColor(tcell.ColorBlack).SetLabel("NeedBody:")
	needParamInput := tview.NewInputField().SetFieldBackgroundColor(tcell.ColorWhite).SetFieldTextColor(tcell.ColorBlack).SetLabel("NeedParam:")

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

	listAddArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive(" ADD API ", tcell.ColorOrange), 2, 0, false).
		AddItem(methodInput, 2, 0, false).
		AddItem(urlInput, 2, 0, false).
		AddItem(needBodyInput, 2, 0, false).
		AddItem(needParamInput, 2, 0, false)

	grid.AddItem(apiList, 0, 0, 2, 1, 0, 0, true).
		AddItem(runBoard, 0, 1, 1, 1, 0, 0, false).
		AddItem(resArea, 1, 1, 1, 1, 0, 0, false).
		AddItem(paramArea, 2, 0, 1, 1, 0, 0, false).
		AddItem(bodyArea, 2, 1, 1, 1, 0, 0, false).
		AddItem(footer, 3, 0, 1, 2, 0, 0, false)

	sendRequest := func(method string, url string, body string) {
		go func() {
			var reqBody io.Reader
			if body != "" {
				reqBody = strings.NewReader(body)
			}
			req, _ := http.NewRequest(method, "http://"+url, reqBody)
			req.Header.Set("Content-Type", "application/json")
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

		switch event.Rune() {
		case 'j':
			if current < total-1 {
				list.SetCurrentItem(current + 1)
			}
			return nil
		case 'k':
			if current > 0 {
				list.SetCurrentItem(current - 1)
			}
			return nil

		case 'f':
			app.SetFocus(filterInput)
			return nil
		case 'a':
			grid.RemoveItem(bodyArea)
			grid.AddItem(apiList, 0, 0, 2, 1, 0, 0, true).
				AddItem(runBoard, 0, 1, 1, 1, 0, 0, false).
				AddItem(resArea, 1, 1, 1, 1, 0, 0, false).
				AddItem(paramArea, 2, 0, 1, 1, 0, 0, false).
				AddItem(listAddArea, 2, 1, 1, 1, 0, 0, false).
				AddItem(footer, 3, 0, 1, 2, 0, 0, false)
			app.SetFocus(methodInput)
			return nil
		case 'd':
			apis = append(apis[:current], apis[current+1:]...)
			updateList("")
			return nil

		}

		if event.Key() == tcell.KeyEnter {
			if response.GetText(true) != "" {
				response.SetText("")
			}
			api := apis[current]
			if !api.NeedBody && !api.NeedParam {
				sendRequest(api.Method, api.Url, "")
				apiBeRunning.SetText("[green]" + api.Method + "[white::-] " + api.Url + " is complete")
				app.SetFocus(response)
			} else {
				if api.NeedParam {
					waitingParam = true
				}
				if api.NeedBody {
					waitingBody = true
				}
				pendingMethod = api.Method
				pendingUrl = api.Url
				apiBeRunning.SetText("[yellow]" + pendingMethod + "[white::-] " + pendingUrl + " is waiting input...")
				if api.NeedParam {
					paramMap := extractUrlParam(pendingUrl)
					var initText string
					for _, param := range paramMap {
						initText += fmt.Sprintf("%v:\n", param)
					}
					paramContent.SetText(initText, false)
					app.SetFocus(paramContent)
				} else if api.NeedBody {
					app.SetFocus(bodyContent)
				}

			}
			return nil
		}
		return event
	})

	paramContent.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyESC {
			waitingParam = false
			pendingMethod = ""
			pendingUrl = ""
			apiBeRunning.SetText("")
			paramContent.SetText("", false)
			app.SetFocus(list)
			return nil
		}
		if event.Key() == tcell.KeyCtrlS && waitingParam {
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
			waitingParam = false
			if waitingBody {
				app.SetFocus(bodyContent)
			} else {
				sendRequest(pendingMethod, pendingUrl, "")
				app.SetFocus(response)
			}
			return nil
		}
		return event
	})

	bodyContent.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyESC {
			waitingBody = false
			pendingMethod = ""
			pendingUrl = ""
			apiBeRunning.SetText("")
			paramContent.SetText("", false)
			app.SetFocus(list)
			return nil
		}
		if event.Key() == tcell.KeyCtrlS && waitingBody {
			waitingBody = false
			body := bodyContent.GetText()
			sendRequest(pendingMethod, pendingUrl, body)
			app.SetFocus(response)
			return nil
		}
		return event
	})

	response.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyESC {
			app.SetFocus(list)
			return nil
		}
		return event
	})

	methodInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEsc {
			grid.RemoveItem(listAddArea)
			grid.AddItem(apiList, 0, 0, 2, 1, 0, 0, true).
				AddItem(runBoard, 0, 1, 1, 1, 0, 0, false).
				AddItem(resArea, 1, 1, 1, 1, 0, 0, false).
				AddItem(paramArea, 2, 0, 1, 1, 0, 0, false).
				AddItem(bodyArea, 2, 1, 1, 1, 0, 0, false).
				AddItem(footer, 3, 0, 1, 2, 0, 0, false)
			app.SetFocus(list)
		}
		if key == tcell.KeyEnter {
			apiAdded.Method = methodInput.GetText()
			app.SetFocus(urlInput)
		}
	})

	urlInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEsc {
			grid.RemoveItem(listAddArea)
			grid.AddItem(apiList, 0, 0, 2, 1, 0, 0, true).
				AddItem(runBoard, 0, 1, 1, 1, 0, 0, false).
				AddItem(resArea, 1, 1, 1, 1, 0, 0, false).
				AddItem(paramArea, 2, 0, 1, 1, 0, 0, false).
				AddItem(bodyArea, 2, 1, 1, 1, 0, 0, false).
				AddItem(footer, 3, 0, 1, 2, 0, 0, false)
			app.SetFocus(list)
		}
		if key == tcell.KeyEnter {
			apiAdded.Url = urlInput.GetText()
			app.SetFocus(needBodyInput)
		}
	})

	needBodyInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEsc {
			grid.RemoveItem(listAddArea)
			grid.AddItem(apiList, 0, 0, 2, 1, 0, 0, true).
				AddItem(runBoard, 0, 1, 1, 1, 0, 0, false).
				AddItem(resArea, 1, 1, 1, 1, 0, 0, false).
				AddItem(paramArea, 2, 0, 1, 1, 0, 0, false).
				AddItem(bodyArea, 2, 1, 1, 1, 0, 0, false).
				AddItem(footer, 3, 0, 1, 2, 0, 0, false)
			app.SetFocus(list)

		}
		if key == tcell.KeyEnter {
			if needBodyInput.GetText() == "true" {
				apiAdded.NeedBody = true
			} else {
				apiAdded.NeedBody = false
			}
			app.SetFocus(needParamInput)
		}
	})

	needParamInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEsc {
			grid.RemoveItem(listAddArea)
			grid.AddItem(apiList, 0, 0, 2, 1, 0, 0, true).
				AddItem(runBoard, 0, 1, 1, 1, 0, 0, false).
				AddItem(resArea, 1, 1, 1, 1, 0, 0, false).
				AddItem(paramArea, 2, 0, 1, 1, 0, 0, false).
				AddItem(bodyArea, 2, 1, 1, 1, 0, 0, false).
				AddItem(footer, 3, 0, 1, 2, 0, 0, false)
			app.SetFocus(list)
		}
		if key == tcell.KeyEnter {
			if needParamInput.GetText() == "true" {
				apiAdded.NeedParam = true
			} else {
				apiAdded.NeedParam = false
			}
			apis = append(apis, apiAdded)
			updateList("")
			grid.RemoveItem(listAddArea)
			grid.AddItem(apiList, 0, 0, 2, 1, 0, 0, true).
				AddItem(runBoard, 0, 1, 1, 1, 0, 0, false).
				AddItem(resArea, 1, 1, 1, 1, 0, 0, false).
				AddItem(paramArea, 2, 0, 1, 1, 0, 0, false).
				AddItem(bodyArea, 2, 1, 1, 1, 0, 0, false).
				AddItem(footer, 3, 0, 1, 2, 0, 0, false)
			app.SetFocus(list)
		}
	})

	if err := app.SetRoot(grid, true).Run(); err != nil {
		panic(err)
	}
}
