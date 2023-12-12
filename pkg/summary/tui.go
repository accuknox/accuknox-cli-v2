package summary

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/accuknox/dev2/api/grpc/v2/summary"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func StartTUI(workload *Workload) {
	app := tview.NewApplication()

	grid := tview.NewGrid().
		SetRows(1, 0, 1, 1).
		SetColumns(15, 40, 0, 85).
		SetBorders(true)
	grid.SetBackgroundColor(tcell.ColorBlack.TrueColor())

	detailsList := tview.NewList().ShowSecondaryText(false)
	detailsList.SetBackgroundColor(tcell.ColorBlack.TrueColor())

	clusterHeader := tview.NewTextView().SetText("Clusters").SetTextAlign(tview.AlignCenter)
	namespaceHeader := tview.NewTextView().SetText("Namespaces/Workloads").SetTextAlign(tview.AlignCenter)
	detailsHeader := tview.NewTextView().SetText("Event Summary").SetTextAlign(tview.AlignCenter)
	eventsHeader := tview.NewTextView().SetText("Event Details").SetTextAlign(tview.AlignCenter)

	grid.AddItem(clusterHeader, 0, 0, 1, 1, 0, 0, false)
	grid.AddItem(namespaceHeader, 0, 1, 1, 1, 0, 0, false)
	grid.AddItem(detailsHeader, 0, 2, 1, 1, 0, 0, false)
	grid.AddItem(eventsHeader, 0, 3, 1, 1, 0, 0, false)
	grid.AddItem(detailsList, 1, 2, 1, 1, 0, 0, false)

	clusterList := tview.NewList()
	for clusterName := range workload.Clusters {
		clusterList.AddItem(clusterName, "", 0, nil)
	}
	clusterList.SetBackgroundColor(tcell.ColorBlack.TrueColor())

	namespaceTree := tview.NewTreeView()
	root := tview.NewTreeNode("Root").SetSelectable(false)
	namespaceTree.SetRoot(root)
	namespaceTree.SetBackgroundColor(tcell.ColorBlack.TrueColor())

	detailsView := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetWordWrap(true)
	detailsView.SetBackgroundColor(tcell.ColorBlack.TrueColor())

	eventDetailsView := tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetWordWrap(true)
	eventDetailsView.SetBackgroundColor(tcell.ColorBlack.TrueColor())

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
			if events, ok := reference.(*WorkloadEvents); ok {
				workloadTypeName := node.GetText()
				populateDetailsList(workloadTypeName, events, detailsList, eventDetailsView)
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
		if events, ok := node.GetReference().(*WorkloadEvents); ok {
			populateDetailsList(node.GetText(), events, detailsList, eventDetailsView)
		}
	})

	if err := app.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

// var DebugLog []string = []string{}
//
// func AddDebugLog(message string) {
// 	DebugLog = append(DebugLog, message)
// }
//
// func PrintDebugLog() {
// 	fmt.Println("called")
// 	fmt.Println(len(DebugLog))
// 	for _, content := range DebugLog {
// 		fmt.Printf("TUI LOG: %s\n", content)
// 	}
// }

func populateNamespaceTree(treeView *tview.TreeView, cluster *Cluster, clusterName string, detailsView, eventDetailsView *tview.TextView) {
	sortedNamespaceNames := sortNamespacesByEvents(cluster)

	// namespaceNames := make([]string, 0, len(cluster.Namespaces))
	// for nsName := range cluster.Namespaces {
	// 	namespaceNames = append(namespaceNames, nsName)
	// }

	root := tview.NewTreeNode(clusterName).SetSelectable(false).SetColor(tcell.ColorGreen)
	treeView.SetRoot(root)

	for _, nsName := range sortedNamespaceNames {
		ns := cluster.Namespaces[nsName]
		nsNode := tview.NewTreeNode(nsName).SetSelectable(true).SetColor(tcell.ColorYellow)
		root.AddChild(nsNode)

		for depName, depEvents := range ns.Deployments {
			eventNode := tview.NewTreeNode(fmt.Sprintf("Deployment: %s", depName)).SetSelectable(true).SetReference(depEvents)
			nsNode.AddChild(eventNode)
		}

		for rsName, rsEvents := range ns.ReplicaSets {
			eventNode := tview.NewTreeNode(fmt.Sprintf("ReplicaSet: %s", rsName)).SetSelectable(true).SetReference(rsEvents)
			nsNode.AddChild(eventNode)
		}

		for ssName, ssEvents := range ns.StatefulSets {
			eventNode := tview.NewTreeNode(fmt.Sprintf("StatefulSet: %s", ssName)).SetSelectable(true).SetReference(ssEvents)
			nsNode.AddChild(eventNode)
		}

		for dsName, dsEvents := range ns.DaemonSets {
			eventNode := tview.NewTreeNode(fmt.Sprintf("DaemonSet: %s", dsName)).SetSelectable(true).SetReference(dsEvents)
			nsNode.AddChild(eventNode)
		}

		for jobName, jobEvents := range ns.Jobs {
			eventNode := tview.NewTreeNode(fmt.Sprintf("Job: %s", jobName)).SetSelectable(true).SetReference(jobEvents)
			nsNode.AddChild(eventNode)
		}

		for cjName, cjEvents := range ns.CronJobs {
			eventNode := tview.NewTreeNode(fmt.Sprintf("CronJob: %s", cjName)).SetSelectable(true).SetReference(cjEvents)
			nsNode.AddChild(eventNode)
		}
	}

	treeView.SetSelectedFunc(func(node *tview.TreeNode) {
		if events, ok := node.GetReference().(*WorkloadEvents); ok {
			workloadTypeName := node.GetText()
			displayWorkloadTypeDetails(workloadTypeName, events, detailsView, eventDetailsView)
		}
	})
}

func displayWorkloadTypeDetails(workloadTypeName string, events *WorkloadEvents, detailsView, eventDetailsView *tview.TextView) {
	summary := createSummary(events)
	detailsView.SetText(fmt.Sprintf("Details for %s:\n%s", workloadTypeName, summary))
	detailsView.ScrollToBeginning()

	detailsView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyDown:
			summary := createSummary(events)
			detailsView.SetText(fmt.Sprintf("Event summary for %s:\n%s", workloadTypeName, summary))
			detailsView.ScrollToBeginning()
		case tcell.KeyRune:
			switch event.Rune() {
			case 'i':
				displayEventDetails("Ingress", events.Events.Ingress, eventDetailsView)
			case 'p':
				displayEventDetails("Process", events.Events.Process, eventDetailsView)
			case 'e':
				displayEventDetails("Egress", events.Events.Egress, eventDetailsView)
			case 'b':
				displayEventDetails("Bind", events.Events.Bind, eventDetailsView)
			case 'f':
				displayEventDetails("File", events.Events.File, eventDetailsView)
			}
		}
		return event
	})
}

func createSummary(events *WorkloadEvents) string {
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "File Events", len(events.Events.File)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Process Events", len(events.Events.Process)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Ingress Events", len(events.Events.Ingress)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Egress Events", len(events.Events.Egress)))
	summary.WriteString(fmt.Sprintf("%-20s: %d\n", "Bind Events", len(events.Events.Bind)))

	return summary.String()
}

func displayEventDetails(eventType string, events interface{}, eventDetailsView *tview.TextView) {
	eventDetails := formatEventDetails(events)
	eventDetailsView.Clear()
	eventDetailsView.SetText(fmt.Sprintf("%s Events:\n%s", eventType, eventDetails))
	eventDetailsView.ScrollToBeginning()
}

func formatEventDetails(eventData interface{}) string {
	eventJSON, err := json.MarshalIndent(eventData, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting details: %v", err)
	}
	return string(eventJSON)
}

func populateDetailsList(workloadTypeName string, events *WorkloadEvents, detailsList *tview.List, eventDetailsView *tview.TextView) {
	detailsList.Clear()

	addEventDetailsItem(detailsList, "File", events.Events.File, eventDetailsView)
	addEventDetailsItem(detailsList, "Process", events.Events.Process, eventDetailsView)
	addEventDetailsItem(detailsList, "Ingress", events.Events.Ingress, eventDetailsView)
	addEventDetailsItem(detailsList, "Egress", events.Events.Egress, eventDetailsView)
	addEventDetailsItem(detailsList, "Bind", events.Events.Bind, eventDetailsView)

	detailsList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		displayEventDetailsFromList(mainText, events, eventDetailsView)
	})
}

func addEventDetailsItem(list *tview.List, eventType string, eventList interface{}, eventDetailsView *tview.TextView) {
	count := getEventCountTUI(eventList)
	list.AddItem(fmt.Sprintf("%s Events (%c) - %d", eventType, eventType[0], count), "", 0, func() {
		displayEventDetails(eventType, eventList, eventDetailsView)
	})
}

func getEventCountTUI(eventList interface{}) int {
	switch v := eventList.(type) {
	case []*summary.ProcessFileEvent:
		return len(v)
	case []*summary.NetworkEvent:
		return len(v)
	default:
		return 0
	}
}

func displayEventDetailsFromList(itemLabel string, events *WorkloadEvents, eventDetailsView *tview.TextView) {
	if strings.Contains(itemLabel, "File Events") {
		displayEventDetails("File", events.Events.File, eventDetailsView)
	} else if strings.Contains(itemLabel, "Process Events") {
		displayEventDetails("Process", events.Events.Process, eventDetailsView)
	} else if strings.Contains(itemLabel, "Ingress Events") {
		displayEventDetails("Ingress", events.Events.Ingress, eventDetailsView)
	} else if strings.Contains(itemLabel, "Egress Events") {
		displayEventDetails("Egress", events.Events.Egress, eventDetailsView)
	} else if strings.Contains(itemLabel, "Bind Events") {
		displayEventDetails("Bind", events.Events.Bind, eventDetailsView)
	}
}
