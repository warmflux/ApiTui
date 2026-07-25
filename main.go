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

	newPrimitive := func(text string) tview.Primitive {
		return tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetText(text)
	}
	grid := tview.NewGrid().SetRows(5, 10, 0).SetColumns(35, 35, 0).SetBorders(true).SetBordersColor(tcell.ColorWhite)

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

	apiBeRunning := tview.NewTextView().SetTextAlign(tview.AlignCenter)
	filterInput := tview.NewInputField().
		SetLabel("filter:").
		SetFieldBackgroundColor(tcell.ColorWhite).
		SetFieldTextColor(tcell.ColorBlack)
	response := tview.NewTextView().SetTextAlign(tview.AlignLeft).SetScrollable(true)
	bodyContent := tview.NewTextArea()
	paramContent := tview.NewTextArea()

	apiList := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("API LIST"), 2, 0, false).
		AddItem(filterInput, 2, 0, false).
		AddItem(list, 0, 1, true)

	runBoard := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("RUNNING API"), 2, 0, false).
		AddItem(apiBeRunning, 0, 1, false)

	resArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("RESPONSE AREA"), 2, 0, false).
		AddItem(response, 0, 1, false)

	paramArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("Param"), 2, 0, false).
		AddItem(paramContent, 0, 1, false)

	bodyArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("Body"), 2, 0, false).
		AddItem(bodyContent, 0, 1, false)

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
		}

		if event.Key() == tcell.KeyEnter {
			if response.GetText(true) != "" {
				response.SetText("")
			}
			api := apis[current]
			if !api.NeedBody && !api.NeedParam {
				sendRequest(api.Method, api.Url, "")
				apiBeRunning.SetText(api.Method + " " + api.Url + " is complete")
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
				apiBeRunning.SetText(pendingMethod + " " + pendingUrl + " is waiting input...")
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

	grid.AddItem(apiList, 0, 0, 3, 1, 0, 0, true).
		AddItem(runBoard, 0, 1, 1, 2, 0, 0, false).
		AddItem(paramArea, 1, 1, 1, 1, 0, 0, false).
		AddItem(bodyArea, 2, 1, 1, 1, 0, 0, false).
		AddItem(resArea, 1, 2, 2, 1, 0, 0, false)

	if err := app.SetRoot(grid, true).Run(); err != nil {
		panic(err)
	}
}
