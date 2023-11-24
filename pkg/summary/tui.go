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

	detailsList := tview.NewList().ShowSecondaryText(false)

	clusterHeader := tview.NewTextView().SetText("Clusters").SetTextColor(tcell.ColorYellow)
	namespaceHeader := tview.NewTextView().SetText("Namespaces/Workloads").SetTextColor(tcell.ColorYellow)
	detailsHeader := tview.NewTextView().SetText("Event Summary").SetTextColor(tcell.ColorYellow)
	eventsHeader := tview.NewTextView().SetText("Event Details").SetTextColor(tcell.ColorYellow)

	grid.AddItem(clusterHeader, 0, 0, 1, 1, 0, 0, false)
	grid.AddItem(namespaceHeader, 0, 1, 1, 1, 0, 0, false)
	grid.AddItem(detailsHeader, 0, 2, 1, 1, 0, 0, false)
	grid.AddItem(eventsHeader, 0, 3, 1, 1, 0, 0, false)
	grid.AddItem(detailsList, 1, 2, 1, 1, 0, 0, false)

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
	grid.AddItem(detailsList, 1, 2, 1, 1, 0, 0, true)
	grid.AddItem(eventDetailsView, 1, 3, 1, 1, 0, 0, false)

	accuKnoxLabel := tview.NewTextView().
		SetText("[::b]AccuKnox[::-]").
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)

	navigationCues := tview.NewTextView().
		SetText("Navigate: Arrows | Select: Click | Exit: Q/Esc").
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
				populateDetailsList(workloadTypeName, wt, detailsList, eventDetailsView)
			}
		}
		children := node.GetChildren()
		if len(children) > 0 && !node.IsExpanded() {
			node.SetExpanded(!node.IsExpanded())
		} else if len(children) == 0 {
			app.SetFocus(detailsList)
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
			populateDetailsList(node.GetText(), wt, detailsList, eventDetailsView)
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

func displayEventDetails(eventType string, events interface{}, eventDetailsView *tview.TextView) {
	eventDetails := formatEventDetails(events)
	eventDetailsView.Clear()
	eventDetailsView.SetText(fmt.Sprintf("%s Events:\n%s", eventType, eventDetails))
	eventDetailsView.ScrollToBeginning()
}

func createEventSummary(events *Events) string {
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "File Events", len(events.File)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Process Events", len(events.Process)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Ingress Events", len(events.Ingress)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Egress Events", len(events.Egress)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Bind Events", len(events.Bind)))

	return summary.String()
}

func formatEventDetails(eventData interface{}) string {
	eventJSON, err := json.MarshalIndent(eventData, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting details: %v", err)
	}
	return string(eventJSON)
}

func populateDetailsList(workloadTypeName string, wt *WorkloadType, detailsList *tview.List, eventDetailsView *tview.TextView) {
	detailsList.Clear()

	detailsList.AddItem(fmt.Sprintf("File Events (f) - %d", len(wt.Events.File)), "", 0, func() {
		displayEventDetails("File", wt.Events.File, eventDetailsView)
	})
	detailsList.AddItem(fmt.Sprintf("Process Events (p) - %d", len(wt.Events.Process)), "", 0, func() {
		displayEventDetails("Process", wt.Events.Process, eventDetailsView)
	})
	detailsList.AddItem(fmt.Sprintf("Ingress Events (i) - %d", len(wt.Events.Ingress)), "", 0, func() {
		displayEventDetails("Ingress", wt.Events.Ingress, eventDetailsView)
	})
	detailsList.AddItem(fmt.Sprintf("Egress Events (e) - %d", len(wt.Events.Egress)), "", 0, func() {
		displayEventDetails("Egress", wt.Events.Egress, eventDetailsView)
	})
	detailsList.AddItem(fmt.Sprintf("Bind Events (b) - %d", len(wt.Events.Bind)), "", 0, func() {
		displayEventDetails("Bind", wt.Events.Bind, eventDetailsView)
	})

	detailsList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		displayEventDetailsFromList(mainText, wt, eventDetailsView)
	})
}

func displayEventDetailsFromList(itemLabel string, wt *WorkloadType, eventDetailsView *tview.TextView) {
	if strings.Contains(itemLabel, "File Events") {
		displayEventDetails("File", wt.Events.File, eventDetailsView)
	} else if strings.Contains(itemLabel, "Process Events") {
		displayEventDetails("Process", wt.Events.Process, eventDetailsView)
	} else if strings.Contains(itemLabel, "Ingress Events") {
		displayEventDetails("Ingress", wt.Events.Ingress, eventDetailsView)
	} else if strings.Contains(itemLabel, "Egress Events") {
		displayEventDetails("Egress", wt.Events.Egress, eventDetailsView)
	} else if strings.Contains(itemLabel, "Bind Events") {
		displayEventDetails("Bind", wt.Events.Bind, eventDetailsView)
	}
}
