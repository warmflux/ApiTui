package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ApiItem struct {
	Method   string
	Url      string
	NeedBody bool
}

var pendingMethod string
var pendingUrl string
var waitingBody bool = false

func sendRequest(method string, url string, body string) string {
	go func() {

	}()

	return ""
}

func main() {
	app := tview.NewApplication()

	newPrimitive := func(text string) tview.Primitive {
		return tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetText(text)
	}
	grid := tview.NewGrid().SetRows(5, 0, 10).SetColumns(35, 0).SetBorders(true).SetBordersColor(tcell.ColorWhite)

	apis := []ApiItem{
		{
			Method:   "GET",
			Url:      "localhost:8080/api/user/:id",
			NeedBody: true,
		},
		{

			Method:   "DELETE",
			Url:      "localhost:8080/api/user/:id",
			NeedBody: true,
		},
	}

	list := tview.NewList().SetHighlightFullLine(true)
	for _, api := range apis {
		list.AddItem(api.Method+" "+api.Url, "", 0, nil)
	}

	apiBeRunning := tview.NewTextView().SetTextAlign(tview.AlignCenter)
	response := tview.NewTextView().SetTextAlign(tview.AlignCenter)
	content := tview.NewTextArea()

	apiList := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("API LIST"), 2, 0, false).
		AddItem(list, 0, 1, true)

	runBoard := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("RUNNING API"), 2, 0, false).
		AddItem(apiBeRunning, 0, 1, false)

	resArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("RESPONSE AREA"), 2, 0, false).
		AddItem(response, 0, 1, false)

	contentArea := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(newPrimitive("INPUT AREA"), 2, 0, false).
		AddItem(content, 0, 1, false)

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
		}

		if event.Key() == tcell.KeyEnter {
			api := apis[current]
			if api.NeedBody == false {
				sendRequest(api.Method, api.Url, "")
			} else {
				waitingBody = true
				pendingMethod = api.Method
				pendingUrl = api.Url
				apiBeRunning.SetText(pendingMethod + " " + pendingUrl + " is waiting body...")
				app.SetFocus(content)
			}
			return nil
		}
		return event
	})

	content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyESC {
			waitingBody = false
			pendingMethod = ""
			pendingUrl = ""
			apiBeRunning.SetText("")
			content.SetText("", false)
			app.SetFocus(list)
		}

		return event

	})

	grid.AddItem(apiList, 0, 0, 3, 1, 0, 0, true).
		AddItem(runBoard, 0, 1, 1, 1, 0, 0, false).
		AddItem(resArea, 1, 1, 1, 1, 0, 0, false).
		AddItem(contentArea, 2, 1, 1, 1, 0, 0, false)

	if err := app.SetRoot(grid, true).Run(); err != nil {
		panic(err)
	}

}
