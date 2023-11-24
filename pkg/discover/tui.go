package discover

import (
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"

	policyType "github.com/accuknox/dev2/discover/pkg/common"
	"github.com/clarketm/json"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

func StartTUI(pf *PolicyForest) {
	app := tview.NewApplication().EnableMouse(true)

	grid := tview.NewGrid().
		SetRows(1, 0, 1, 1).
		SetColumns(25, 0, 85).
		SetBorders(true)

	policyTree := tview.NewTreeView().
		SetRoot(tview.NewTreeNode("Policy Tree")).
		SetCurrentNode(tview.NewTreeNode("Policy Tree"))

	namespaceList := tview.NewList()

	policyDetailsView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetScrollable(true)

	for ns := range pf.Namespaces {
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
		populatePolicyTree(policyTree, mainText, pf)
		app.SetFocus(policyTree)
	})

	policyTree.SetChangedFunc(func(node *tview.TreeNode) {
		switch ref := node.GetReference().(type) {
		case *networkingv1.NetworkPolicy:
			policyDetailsView.SetText(networkPolicyToString(ref))
		case *policyType.KubeArmorPolicy:
			policyDetailsView.SetText(kubearmorPolicyToString(ref))
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
		// TODO: Fallback to standard display in case TUI fails
		log.WithError(err).Errorf("failed to start TUI: %v", err)
	}
}

func populatePolicyTree(tree *tview.TreeView, namespace string, pf *PolicyForest) {
	root := tree.GetRoot()
	root.ClearChildren()

	nb := pf.Namespaces[namespace]

	kubearmorNode := tview.NewTreeNode("KubeArmor Policies").
		SetColor(tcell.ColorYellow).
		SetSelectable(false)
	root.AddChild(kubearmorNode)

	for action, policies := range nb.KubearmorPolicies.Actions {
		actionNode := tview.NewTreeNode(fmt.Sprintf("Action: %s (%d)", action, len(policies))).
			SetColor(tcell.ColorGreen).SetSelectable(false)
		kubearmorNode.AddChild(actionNode)
		for _, policy := range policies {
			policyNode := tview.NewTreeNode(policy.Metadata.Name).
				SetReference(policy)
			actionNode.AddChild(policyNode)
		}
	}

	for label, policies := range nb.KubearmorPolicies.Labels {
		labelNode := tview.NewTreeNode(fmt.Sprintf("Label: %s (%d)", label, len(policies))).
			SetColor(tcell.ColorGreen).SetSelectable(false)
		kubearmorNode.AddChild(labelNode)
		for _, policy := range policies {
			policyNode := tview.NewTreeNode(policy.Metadata.Name).
				SetReference(policy)
			labelNode.AddChild(policyNode)
		}
	}

	networkPolicyNode := tview.NewTreeNode("Network Policies").
		SetColor(tcell.ColorYellow).
		SetSelectable(false)
	root.AddChild(networkPolicyNode)

	for typ, policies := range nb.NetworkPolicies.Types {
		typeNode := tview.NewTreeNode(fmt.Sprintf("Type: %s (%d)", typ, len(policies))).
			SetColor(tcell.ColorGreen).SetSelectable(false)
		networkPolicyNode.AddChild(typeNode)
		for _, policy := range policies {
			policyNode := tview.NewTreeNode(policy.ObjectMeta.Name).
				SetReference(policy)
			typeNode.AddChild(policyNode)
		}
	}

	for protocol, policies := range nb.NetworkPolicies.Protocols {
		protocolNode := tview.NewTreeNode(fmt.Sprintf("Protocol: %s (%d)", protocol, len(policies))).
			SetColor(tcell.ColorGreen).SetSelectable(false)
		networkPolicyNode.AddChild(protocolNode)
		for _, policy := range policies {
			policyNode := tview.NewTreeNode(policy.ObjectMeta.Name).
				SetReference(policy)
			protocolNode.AddChild(policyNode)
		}
	}

	tree.SetCurrentNode(root)
}

func kubearmorPolicyToString(policy *policyType.KubeArmorPolicy) string {
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

func networkPolicyToString(policy *networkingv1.NetworkPolicy) string {
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
