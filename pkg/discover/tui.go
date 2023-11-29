package discover

import (
	"fmt"
	"os"
	"strings"

	"github.com/clarketm/json"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sigs.k8s.io/yaml"

	policyType "github.com/accuknox/dev2/discover/pkg/common"
	log "github.com/sirupsen/logrus"
	terminal "golang.org/x/term"
	networkingv1 "k8s.io/api/networking/v1"
)

// Track the policies that are already saved, and dont let users save the same policy again
var savedPolicies = make(map[string]bool)

// Set of all policies to be dumped
var policiesToSave []policySaveInfo

// policySaveInfo is a struct that holds the information about a policy that needs to be saved
type policySaveInfo struct {
	Policy     interface{}
	PolicyType string
	Namespace  string
	Name       string
}

func StartTUI(pf *PolicyForest) {
	app := tview.NewApplication().EnableMouse(true)

	grid := tview.NewGrid().
		SetRows(1, 0, 1, 1).
		SetColumns(25, 0, 85).
		SetBorders(true)
	grid.SetBackgroundColor(tcell.ColorBlack.TrueColor()) // hardcore black background, will override any terminal default color

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
		SetText("Navigate: Arrows | Select: Click | Save: S | Exit: Q/Esc").
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
		if event.Rune() == 's' {
			currentNode := policyTree.GetCurrentNode()
			if currentNode == nil || currentNode.GetReference() == nil {
				policyDetailsView.SetText("Please select a policy to save")
				return nil
			}
			policyName := currentNode.GetText()
			if savedPolicies[policyName] {
				policyDetailsView.SetText("Policy is already saved.")
				return nil
			}
			showDetailsDialog(app, grid, policyTree, namespaceList)
		}

		return event
	})

	if err := app.SetRoot(grid, true).Run(); err != nil {
		// TODO: Fallback to standard display in case TUI fails
		log.WithError(err).Errorf("failed to start TUI: %v", err)
	}

	dumpPolicies()
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

func saveCurrentPolicy(policyTree *tview.TreeView, namespaceList *tview.List) {
	currentNode := policyTree.GetCurrentNode()
	policy := currentNode.GetReference()
	namespaceIdx := namespaceList.GetCurrentItem()
	namespaceTxt, _ := namespaceList.GetItemText(namespaceIdx)
	policyName := currentNode.GetText()

	var polType string
	if strings.HasPrefix(policyName, "autopol-system-") {
		polType = "kubearmor_policy"
	} else {
		polType = "network_policy"
	}

	policiesToSave = append(policiesToSave, policySaveInfo{
		Policy:     policy,
		PolicyType: polType,
		Namespace:  namespaceTxt,
		Name:       policyName,
	})
}

func showDetailsDialog(app *tview.Application, grid *tview.Grid, policyTree *tview.TreeView, namespaceList *tview.List) {
	currentNode := policyTree.GetCurrentNode()
	policy := currentNode.GetReference()
	policyDetails := ""

	switch p := policy.(type) {
	case *networkingv1.NetworkPolicy:
		policyDetails = networkPolicyToString(p)
	case *policyType.KubeArmorPolicy:
		policyDetails = kubearmorPolicyToString(p)
	}

	detailsTextView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetTextAlign(tview.AlignLeft).
		SetText(policyDetails)

	detailsTextView.SetBorder(true)
	detailsTextView.SetTitle("DISCOVERED POLICY")
	detailsTextView.SetBackgroundColor(tcell.ColorBlack.TrueColor())

	detailsTextView.SetText(policyDetails)

	saveButton := tview.NewButton("Save").SetSelectedFunc(func() {
		currentNode := policyTree.GetCurrentNode()
		if currentNode == nil || currentNode.GetReference() == nil {
			detailsTextView.SetText("Please select a policy to save.")
			return
		}
		policyName := currentNode.GetText()
		if savedPolicies[policyName] {
			detailsTextView.SetText("Policy is already saved.")
			return
		}

		saveCurrentPolicy(policyTree, namespaceList)
		savedPolicies[policyName] = true
		app.SetRoot(grid, true).SetFocus(grid)
	})
	cancelButton := tview.NewButton("Cancel").SetSelectedFunc(func() {
		app.SetRoot(grid, true).SetFocus(grid)
	})

	width, _, err := terminal.GetSize(0)
	if err != nil {
		return
	}

	requiredWidth := width / 2

	buttonHeight := 3

	buttonsFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(saveButton, 0, 1, true).
		AddItem(cancelButton, 0, 1, true).
		AddItem(nil, 0, 1, false)

	detailsBoxInner := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(detailsTextView, 0, 1, true)

	dialogFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(detailsBoxInner, 0, 1, true).
		AddItem(buttonsFlex, buttonHeight, 1, false)

	centeredDialogFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(dialogFlex, requiredWidth, 1, true).
		AddItem(nil, 0, 1, false)

	app.SetRoot(centeredDialogFlex, true)
	app.SetFocus(saveButton)
}

func dumpPolicies() {
	for _, p := range policiesToSave {
		nsDirPath := fmt.Sprintf("knoxctl_out/discovered/policies/%s/%s", p.PolicyType, p.Namespace)
		filename := fmt.Sprintf("%s.yaml", p.Name)

		err := os.MkdirAll(nsDirPath, 0750)
		if err != nil {
			log.Errorf("Failed to create directory %s: %v", nsDirPath, err)
			continue
		}

		if p.PolicyType == "kubearmor_policy" {
			policy, ok := p.Policy.(*policyType.KubeArmorPolicy)
			if !ok {
				log.Errorf("Invalid policy type for policy: %s", p.Name)
				continue
			}
			err = writePolicyToFile(policy, nsDirPath, filename)
		} else if p.PolicyType == "network_policy" {
			policy, ok := p.Policy.(*networkingv1.NetworkPolicy)
			if !ok {
				log.Errorf("Invalid policy type for policy: %s", p.Name)
				continue
			}
			err = writeNetworkPolicyToFile(policy, nsDirPath, filename)
		}
		if err != nil {
			fmt.Printf("Failed to save policy %s: %v\n", p.Name, err)
		} else {
			fmt.Printf("Saved policy %s\n", p.Name)
		}
	}
}
