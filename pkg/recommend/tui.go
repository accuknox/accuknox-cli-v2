package recommend

import (
	"fmt"
	"strings"

	"github.com/clarketm/json"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sigs.k8s.io/yaml"

	policyType "github.com/accuknox/dev2/hardening/pkg/types"
)

func StartTUI(pb *PolicyBucket) {
	app := tview.NewApplication().EnableMouse(true)

	grid := tview.NewGrid().
		SetRows(1, 0, 1, 1).
		SetColumns(25, 0, 90).
		SetBorders(true)
	grid.SetBackgroundColor(tcell.ColorBlack.TrueColor())

	policyTree := tview.NewTreeView().
		SetRoot(tview.NewTreeNode("Policy Tree")).
		SetCurrentNode(tview.NewTreeNode("Policy Tree"))
	policyTree.SetBackgroundColor(tcell.ColorBlack.TrueColor())

	namespaceList := tview.NewList()
	namespaceList.SetBackgroundColor(tcell.ColorBlack.TrueColor())

	policyDetailsView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetScrollable(true)
	policyDetailsView.SetBackgroundColor(tcell.ColorBlack.TrueColor())

	for ns := range pb.Namespaces {
		namespaceList.AddItem(ns, "", 0, nil)
	}

	grid.AddItem(tview.NewTextView().SetText("Namespaces").SetTextAlign(tview.AlignCenter), 0, 0, 1, 1, 0, 0, false)
	grid.AddItem(tview.NewTextView().SetText("Tree View").SetTextAlign(tview.AlignCenter), 0, 1, 1, 1, 0, 0, false)
	grid.AddItem(tview.NewTextView().SetText("Policies").SetTextAlign(tview.AlignCenter), 0, 2, 1, 1, 0, 0, false)

	grid.AddItem(namespaceList, 1, 0, 1, 1, 0, 0, true)
	grid.AddItem(policyTree, 1, 1, 1, 1, 0, 0, true)
	grid.AddItem(policyDetailsView, 1, 2, 1, 1, 0, 0, false)

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

	grid.AddItem(navFlex, 3, 0, 1, 3, 0, 100, false)

	namespaceList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		populatePolicyTree(policyTree, mainText, pb)
		app.SetFocus(policyTree)
	})

	policyTree.SetSelectedFunc(func(node *tview.TreeNode) {
		if policies, ok := node.GetReference().([]*policyType.KubeArmorPolicy); ok {
			policyDetails := ""
			for _, policy := range policies {
				policyDetails += policyToString(policy) + "\n" + strings.Repeat("_", 90) + "\n"
			}
			policyDetailsView.SetText(policyDetails)
			app.SetFocus(policyDetailsView)
		}
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			app.Stop()
		}
		if event.Rune() == 'q' || event.Key() == tcell.KeyEscape {
			app.Stop()
		}
		if event.Key() == tcell.KeyLeft {
			if app.GetFocus() == policyDetailsView {
				app.SetFocus(policyTree)
			} else {
				app.SetFocus(namespaceList)
			}
		}
		return event
	})

	if err := app.SetRoot(grid, true).Run(); err != nil {
		fmt.Println("Failed to start TUI: \nStarting native terminal view.", err)
		printTable(pb)
	}
}

func populatePolicyTree(tree *tview.TreeView, namespace string, pb *PolicyBucket) {
	root := tree.GetRoot()
	root.ClearChildren()

	ab := pb.Namespaces[namespace]

	actionNode := tview.NewTreeNode("Actions").
		SetColor(tcell.ColorYellow).
		SetSelectable(false)
	root.AddChild(actionNode)
	for action, policies := range ab.Actions {
		actionChild := tview.NewTreeNode(fmt.Sprintf("%s (%d)", action, len(policies))).
			SetReference(policies)
		actionNode.AddChild(actionChild)
	}

	labelNode := tview.NewTreeNode("Labels").
		SetColor(tcell.ColorYellow).
		SetSelectable(false)
	root.AddChild(labelNode)
	for label, policies := range ab.Labels {
		labelChild := tview.NewTreeNode(fmt.Sprintf("%s (%d)", label, len(policies))).
			SetReference(policies)
		labelNode.AddChild(labelChild)
	}

	severityNode := tview.NewTreeNode("Severities").
		SetColor(tcell.ColorYellow).
		SetSelectable(false)
	root.AddChild(severityNode)
	for severity, policies := range ab.Severties {
		severityChild := tview.NewTreeNode(fmt.Sprintf("Severity %v (%d)", severity, len(policies))).
			SetReference(policies)
		severityNode.AddChild(severityChild)
	}

	tagNode := tview.NewTreeNode("Tags").
		SetColor(tcell.ColorYellow).
		SetSelectable(false)
	root.AddChild(tagNode)
	for tag, policies := range ab.Tags {
		tagChild := tview.NewTreeNode(fmt.Sprintf("%s (%d)", tag, len(policies))).
			SetReference(policies)
		tagNode.AddChild(tagChild)
	}

	tree.SetCurrentNode(root)
}

func policyToString(policy *policyType.KubeArmorPolicy) string {
	jsonBytes, err := json.Marshal(policy)
	if err != nil {
		return fmt.Sprintf("error marshalling policy to JSON: %v", err)
	}

	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return fmt.Sprintf("error marshalling policy to YAML: %v", err)
	}

	return string(yamlBytes)
}
