package summary

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func StartTUI(workload *Workload) {
	app := tview.NewApplication()

	grid := tview.NewGrid().
		SetRows(1, 0, 1, 1).
		SetColumns(15, 40, 0, 85).
		SetBorders(true)

	clusterHeader := tview.NewTextView().SetText("Clusters").SetTextColor(tcell.ColorYellow)
	namespaceHeader := tview.NewTextView().SetText("Namespaces/Workloads").SetTextColor(tcell.ColorYellow)
	detailsHeader := tview.NewTextView().SetText("Event Summary").SetTextColor(tcell.ColorYellow)
	eventsHeader := tview.NewTextView().SetText("Event Details").SetTextColor(tcell.ColorYellow)

	grid.AddItem(clusterHeader, 0, 0, 1, 1, 0, 0, false)
	grid.AddItem(namespaceHeader, 0, 1, 1, 1, 0, 0, false)
	grid.AddItem(detailsHeader, 0, 2, 1, 1, 0, 0, false)
	grid.AddItem(eventsHeader, 0, 3, 1, 1, 0, 0, false)

	clusterList := tview.NewList()
	for clusterName := range workload.Clusters {
		clusterList.AddItem(clusterName, "", 0, nil)
	}

	namespaceTree := tview.NewTreeView()
	root := tview.NewTreeNode("Root").SetSelectable(false)
	namespaceTree.SetRoot(root)

	detailsView := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetWordWrap(true)
	eventDetailsView := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetWordWrap(true)

	grid.AddItem(clusterList, 1, 0, 1, 1, 0, 0, true)
	grid.AddItem(namespaceTree, 1, 1, 1, 1, 0, 0, false)
	grid.AddItem(detailsView, 1, 2, 1, 1, 0, 0, false)
	grid.AddItem(eventDetailsView, 1, 3, 1, 1, 0, 0, false)

	accuKnoxLabel := tview.NewTextView().
		SetText("[::b]AccuKnox[::-]").
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)

	navigationCues := tview.NewTextView().
		SetText("Navigate: Arrows | Select: Enter | Exit: Q/Esc").
		SetTextAlign(tview.AlignRight).
		SetDynamicColors(true)

	navFlex := tview.NewFlex().
		AddItem(accuKnoxLabel, 0, 1, false).
		AddItem(tview.NewBox(), 0, 1, false).
		AddItem(navigationCues, 0, 1, false)

	grid.AddItem(navFlex, 3, 0, 1, 4, 0, 100, false)

	var currentSelectedFunc func(*tview.TreeNode)
	selectedFunc := func(node *tview.TreeNode) {
		reference := node.GetReference()
		if reference != nil {
			if wt, ok := reference.(*WorkloadType); ok {
				workloadTypeName := node.GetText()
				displayWorkloadTypeDetails(workloadTypeName, wt, detailsView, eventDetailsView)
			}
		}
		children := node.GetChildren()
		if len(children) > 0 && !node.IsExpanded() {
			node.SetExpanded(!node.IsExpanded())
		} else if len(children) == 0 {
			app.SetFocus(detailsView)
		}
	}

	clusterList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		defer func() {
			detailsView.Clear()
			eventDetailsView.Clear()
		}()

		namespaceTree.GetRoot().ClearChildren()

		populateNamespaceTree(namespaceTree, workload.Clusters[mainText], mainText, detailsView, eventDetailsView)
		app.SetFocus(namespaceTree)
	})

	namespaceTree.SetSelectedFunc(selectedFunc)
	currentSelectedFunc = selectedFunc

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			node := namespaceTree.GetCurrentNode()
			if node != nil {
				currentSelectedFunc(node)
			}
			return nil
		}
		if event.Key() == tcell.KeyLeft {
			if app.GetFocus() == detailsView {
				app.SetFocus(namespaceTree)
			} else if app.GetFocus() == namespaceTree {
				app.SetFocus(clusterList)
			}
			return nil
		}
		if event.Key() == tcell.KeyRight {
			if app.GetFocus() == clusterList {
				app.SetFocus(namespaceTree)
			} else if app.GetFocus() == namespaceTree {
				app.SetFocus(detailsView)
			}
			return nil
		}
		if event.Key() == tcell.KeyESC || event.Rune() == 'q' {
			app.Stop()
			return nil
		}

		return event
	})

	namespaceTree.SetChangedFunc(func(node *tview.TreeNode) {
		if wt, ok := node.GetReference().(*WorkloadType); ok {
			displayWorkloadTypeDetails(node.GetText(), wt, detailsView, eventDetailsView)
		}
	})

	if err := app.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func populateNamespaceTree(treeView *tview.TreeView, cluster *Cluster, clusterName string, detailsView, eventDetailsView *tview.TextView) {
	sortedNamespaceNames := sortNamespacesByEvents(cluster)

	root := tview.NewTreeNode(clusterName).SetSelectable(false)
	treeView.SetRoot(root)

	for _, nsName := range sortedNamespaceNames {
		ns := cluster.Namespaces[nsName]
		nsNode := tview.NewTreeNode(nsName).SetSelectable(false)
		root.AddChild(nsNode)

		for wtName, wt := range ns.WorkloadTypes {
			if wt.GetEvents().TotalEvents() > 0 { // only add a child if there are events
				wtNode := tview.NewTreeNode(wtName).SetSelectable(true).SetReference(wt)
				nsNode.AddChild(wtNode)
			}
		}
	}

	treeView.SetSelectedFunc(func(node *tview.TreeNode) {
		if wt, ok := node.GetReference().(*WorkloadType); ok {
			workloadTypeName := node.GetText()
			displayWorkloadTypeDetails(workloadTypeName, wt, detailsView, eventDetailsView)
		}
	})
}

func displayWorkloadTypeDetails(workloadTypeName string, wt *WorkloadType, detailsView, eventDetailsView *tview.TextView) {
	summary := createEventSummary(wt.Events)
	detailsView.SetText(fmt.Sprintf("Details for %s:\n%s", workloadTypeName, summary))
	detailsView.ScrollToBeginning()

	detailsView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyDown:
			summary := createEventSummary(wt.Events)
			detailsView.SetText(fmt.Sprintf("Event summary for %s:\n%s", workloadTypeName, summary))
			detailsView.ScrollToBeginning()
		case tcell.KeyRune:
			switch event.Rune() {
			case 'i':
				displayEventDetails("Ingress", wt.Events.Ingress, eventDetailsView)
			case 'p':
				displayEventDetails("Process", wt.Events.Process, eventDetailsView)
			case 'e':
				displayEventDetails("Egress", wt.Events.Egress, eventDetailsView)
			case 'b':
				displayEventDetails("Bind", wt.Events.Bind, eventDetailsView)
			case 'f':
				displayEventDetails("File", wt.Events.File, eventDetailsView)
			}
		}
		return event
	})
}

func displayEventDetails(eventType string, eventData interface{}, eventDetailsView *tview.TextView) {
	header := fmt.Sprintf("[::b]%s Events[::-]\n", eventType)
	details := formatEventDetails(eventData)
	eventDetailsView.SetText(header + details)
	eventDetailsView.ScrollToBeginning()
}

func createEventSummary(events *Events) string {
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "File Events (f)", len(events.File)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Process Events (p)", len(events.Process)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Ingress Events (i)", len(events.Ingress)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Egress Events (e)", len(events.Egress)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Bind Events (b)", len(events.Bind)))

	return summary.String()
}

func formatEventDetails(eventData interface{}) string {
	eventJSON, err := json.MarshalIndent(eventData, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting details: %v", err)
	}
	return string(eventJSON)
}
