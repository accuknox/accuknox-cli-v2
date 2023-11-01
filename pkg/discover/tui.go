package discover

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/clarketm/json"
	"github.com/gizak/termui/v3/widgets"
	"sigs.k8s.io/yaml"

	ui "github.com/gizak/termui/v3"
	log "github.com/sirupsen/logrus"
)

type PolicyDisplay struct {
	data               []string     // List of policies (each policy is a string)
	currentPolicyIndex int          // Index pointing to current policy in display
	scrollOffset       int          // Number of lines scrolled vertically in current policy view
	width              int          // Width of display box
	savedPolicies      map[int]bool // Keeping track of saved policies
	linesPerPage       int          // Lines of policy to display at once
	page               int          // Current page of the policy being displayed
	leftArrowColor     ui.Color     // Nav cue color (left arrow)
	rightArrowColor    ui.Color     // Nav cue color (right arrow)
}

func NewPolicyDisplay(data []string) *PolicyDisplay {
	return &PolicyDisplay{
		data:            data,
		savedPolicies:   make(map[int]bool),
		width:           80,
		linesPerPage:    40,
		leftArrowColor:  ui.ColorWhite,
		rightArrowColor: ui.ColorWhite,
	}
}

func (pd *PolicyDisplay) extractPolicyParts(policy string) (string, string) {
	parts := strings.SplitN(policy, "|", 4)
	name := strings.Split(parts[0], ":")[1]
	namespace := strings.Split(parts[1], ":")[1]
	kind := strings.Split(parts[2], ":")[1]
	metadata := fmt.Sprintf("Name: %s\nNamespace: %s\nKind: %s\n\n", name, namespace, kind)
	content := parts[3]
	return metadata, content
}

func (pd *PolicyDisplay) renderPolicyBox(policy string, yPos int) int {
	metadata, content := pd.extractPolicyParts(policy)

	allLines := strings.Split(content, "\n")
	totalPolicyLines := len(allLines)
	startIndex := pd.page * pd.linesPerPage
	endIndex := (pd.page+1)*pd.linesPerPage + pd.scrollOffset
	if startIndex > len(allLines)-1 {
		startIndex = len(allLines) - 1
	}
	if endIndex > len(allLines) {
		endIndex = len(allLines)
	}
	lines := allLines[startIndex:endIndex]

	if pd.scrollOffset > 0 && pd.scrollOffset < len(lines) {
		content = strings.Join(lines[pd.scrollOffset:], "\n")
	}

	title := " AccuKnox "
	padding := (pd.width - len(title) - 2) / 2
	titleBox := widgets.NewParagraph()
	titleBox.Text = title
	titleBox.TextStyle.Fg = ui.ColorWhite
	titleBox.TextStyle.Modifier = ui.ModifierBold
	titleBox.Border = false
	titleBox.WrapText = false
	titleBox.SetRect(padding, yPos, padding+len(title)+2, yPos+1)
	ui.Render(titleBox)
	yPos++

	// Metadata section
	metaBox := widgets.NewParagraph()
	metaBox.Text = metadata
	metaBox.TextStyle.Fg = ui.ColorBlue
	metaBox.TextStyle.Modifier = ui.ModifierBold
	metaBox.Border = false
	metaBox.WrapText = false
	metaHeight := len(strings.Split(metadata, "\n"))
	metaBox.SetRect(0, yPos, pd.width, yPos+metaHeight)
	ui.Render(metaBox)
	yPos += metaHeight

	// Navigation cues
	leftArrowWidget := widgets.NewParagraph()
	rightArrowWidget := widgets.NewParagraph()

	leftArrow := "<"
	rightArrow := ">"

	navCues := fmt.Sprintf(" [%d|%d] [Lines: %d] \t\t", pd.currentPolicyIndex+1, len(pd.data), totalPolicyLines)

	navWidth := len(navCues) + 2

	leftPadding := (pd.width - navWidth) / 2
	navBoxStart := leftPadding + 2
	navBoxEnd := navBoxStart + len(navCues)

	navBox := widgets.NewParagraph()
	navBox.Text = navCues
	navBox.TextStyle.Fg = ui.ColorWhite
	navBox.Border = false
	navBox.WrapText = false
	navBox.SetRect(navBoxStart, yPos, navBoxEnd, yPos+1)

	leftArrowWidget.Text = leftArrow
	rightArrowWidget.Text = rightArrow

	leftArrowWidget.TextStyle.Modifier = ui.ModifierBold
	rightArrowWidget.TextStyle.Modifier = ui.ModifierBold

	if pd.leftArrowColor == ui.ColorGreen {
		leftArrowWidget.TextStyle.Fg = ui.ColorGreen
	} else {
		leftArrowWidget.TextStyle.Fg = ui.ColorWhite
	}

	if pd.rightArrowColor == ui.ColorGreen {
		rightArrowWidget.TextStyle.Fg = ui.ColorGreen
	} else {
		rightArrowWidget.TextStyle.Fg = ui.ColorWhite
	}

	leftArrowWidget.Border = false
	rightArrowWidget.Border = false
	leftArrowWidget.WrapText = false
	rightArrowWidget.WrapText = false
	leftArrowWidget.SetRect(leftPadding, yPos, leftPadding+1, yPos+1)
	rightArrowWidget.SetRect(navBoxEnd+1, yPos, navBoxEnd+2, yPos+1)

	ui.Render(navBox, leftArrowWidget, rightArrowWidget)
	yPos++

	// Content section
	contentBox := widgets.NewParagraph()
	contentBox.Text = content
	contentBox.Border = true
	contentBox.WrapText = true
	fixedHeight := 30
	contentBox.SetRect(0, yPos, pd.width, yPos+fixedHeight)
	ui.Render(contentBox)
	yPos += fixedHeight + 2

	return yPos
}

func (pd *PolicyDisplay) savePolicyToFile(policy string) string {
	metadata, content := pd.extractPolicyParts(policy)
	name := strings.TrimSpace(strings.Split(metadata, "\n")[0][6:])
	namespace := strings.TrimSpace(strings.Split(metadata, "\n")[1][11:])
	filePath := "out/discovered"
	fileName := fmt.Sprintf("%s/%s_%s.yaml", filePath, name, namespace)

	err := os.MkdirAll(filePath, os.ModePerm)
	if err != nil {
		log.WithError(err).Error("failed to create directory")
	}

	if json.Valid([]byte(content)) {
		var jsonObj interface{}
		err := json.Unmarshal([]byte(content), &jsonObj)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal")
		} else {
			yamlContent, err := yaml.Marshal(jsonObj)
			if err != nil {
				log.WithError(err).Error("failed to convert JSON to YAML")
			} else {
				content = string(yamlContent)
			}
		}
	}

	err = os.WriteFile(fileName, []byte(content), 0644)
	if err != nil {
		log.WithError(err).Error("failed to write/save the policy")
	}

	pd.savedPolicies[pd.currentPolicyIndex] = true

	return "Saved policy " + fileName
}

func (pd *PolicyDisplay) getCueText() string {
	if pd.savedPolicies[pd.currentPolicyIndex] {
		metadata, _ := pd.extractPolicyParts(pd.data[pd.currentPolicyIndex])
		name := strings.TrimSpace(strings.Split(metadata, "\n")[0][6:])
		namespace := strings.TrimSpace(strings.Split(metadata, "\n")[1][11:])
		return "Saved policy to out/discovered/" + name + "_" + namespace + ".yaml"
	}
	return "Press 's' to save this policy"
}

func (pd *PolicyDisplay) Display(p Options) error {
	if len(pd.data) == 0 {
		log.WithFields(log.Fields{
			"kind":           p.Kind,
			"format":         p.Format,
			"namespace":      p.Namespace,
			"labels":         p.Labels,
			"fromSource":     p.Source,
			"gRPC":           p.GRPC,
			"includeNetwork": p.IncludeNetwork,
		}).Warn("no policies found")
		return nil
	}

	if err := ui.Init(); err != nil {
		log.Warnf("failed to initialize termui: %v", err)
		log.Warn("falling back to standard display...")
		for idx, policy := range pd.data {
			prettyOutput := prettifyPolicy(policy, idx+1, len(pd.data))
			fmt.Println(prettyOutput)
		}
		return nil
	}
	defer ui.Close()

	renderCurrentPolicy := func() {
		ui.Clear()
		yPos := 0
		yPos = pd.renderPolicyBox(pd.data[pd.currentPolicyIndex], yPos)

		cueBox := widgets.NewParagraph()
		cueBox.Text = pd.getCueText()
		cueBox.Border = false
		cueBox.WrapText = false
		if pd.savedPolicies[pd.currentPolicyIndex] {
			cueBox.TextStyle.Fg = ui.ColorGreen
		} else {
			cueBox.TextStyle.Fg = ui.ColorRed
		}
		cueBox.SetRect(0, yPos, pd.width, yPos+1)
		ui.Render(cueBox)
	}

	renderCurrentPolicy()

	for e := range ui.PollEvents() {
		needsRedraw := false

		switch e.ID {
		case "q", "<C-c>":
			return nil
		case "<Up>":
			if pd.scrollOffset > 0 {
				pd.scrollOffset--
				needsRedraw = true
			} else if pd.page > 0 {
				pd.page--
				pd.scrollOffset = pd.linesPerPage - 1
				needsRedraw = true
			}
		case "<Down>":
			_, terminalHeight := ui.TerminalDimensions()
			maxVisibleLines := terminalHeight - 10
			totalLines := len(strings.Split(pd.data[pd.currentPolicyIndex], "\n"))
			linesLeft := totalLines - (pd.page*pd.linesPerPage + pd.scrollOffset)

			if pd.scrollOffset < pd.linesPerPage-1 && pd.scrollOffset < maxVisibleLines-1 && linesLeft > 1 {
				pd.scrollOffset++
				needsRedraw = true
			} else if (pd.page+1)*pd.linesPerPage < totalLines && linesLeft > pd.linesPerPage {
				pd.page++
				pd.scrollOffset = 0
				needsRedraw = true
			}
		case "<Right>":
			if pd.currentPolicyIndex < len(pd.data)-1 {
				pd.rightArrowColor = ui.ColorGreen
				renderCurrentPolicy()
				time.Sleep(150 * time.Millisecond)
				pd.rightArrowColor = ui.ColorWhite

				pd.currentPolicyIndex++
				pd.scrollOffset = 0
				pd.page = 0
				needsRedraw = true
			}

		case "<Left>":
			if pd.currentPolicyIndex > 0 {
				pd.leftArrowColor = ui.ColorGreen
				renderCurrentPolicy()
				time.Sleep(150 * time.Millisecond)
				pd.leftArrowColor = ui.ColorWhite

				pd.currentPolicyIndex--
				pd.scrollOffset = 0
				pd.page = 0
				needsRedraw = true
			}
		case "s":
			if !pd.savedPolicies[pd.currentPolicyIndex] {
				pd.savePolicyToFile(pd.data[pd.currentPolicyIndex])
			}
			needsRedraw = true
		}

		if needsRedraw {
			renderCurrentPolicy()
		}
	}

	return nil
}
