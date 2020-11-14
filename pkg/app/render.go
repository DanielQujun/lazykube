package app

import (
	"bytes"
	"fmt"
	"github.com/TNK-Studio/lazykube/pkg/gui"
	"github.com/TNK-Studio/lazykube/pkg/kubecli"
	"github.com/TNK-Studio/lazykube/pkg/log"
	"github.com/TNK-Studio/lazykube/pkg/utils"
	"github.com/gookit/color"
	"io"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	"strings"
)

const (
	OptSeparator       = "   "
	navigationPathJoin = " + "
	logsTail           = "200"
)

var (
	// Todo: use state to control.
	activeView *gui.View

	navigationIndex     int
	activeNavigationOpt string

	functionViews     = []string{clusterInfoViewName, namespaceViewName, serviceViewName, deploymentViewName, podViewName}
	viewNavigationMap = map[string][]string{
		clusterInfoViewName: []string{"Nodes", "Top Nodes"},
		namespaceViewName:   []string{"Config", "Deployments", "Pods"},
		serviceViewName:     []string{"Config", "Pods", "Pods Log", "Top Pods"},
		deploymentViewName:  []string{"Config", "Pods", "Pods Log", "Describe", "Top Pods"},
		podViewName:         []string{"Log", "Config", "Top", "Describe"},
	}

	detailRenderMap = map[string]func(gui *gui.Gui, view *gui.View) error{
		navigationPath(clusterInfoViewName, "Nodes"):     clusterNodesRender,
		navigationPath(clusterInfoViewName, "Top Nodes"): topNodesRender,
		navigationPath(namespaceViewName, "Deployments"): deploymentRender,
		navigationPath(namespaceViewName, "Pods"):        podRender,
		navigationPath(namespaceViewName, "Config"):      configRender,
		navigationPath(serviceViewName, "Config"):        configRender,
		navigationPath(serviceViewName, "Pods"):          labelsPodsRender,
		navigationPath(serviceViewName, "Pods Log"):      podsLogsRender,
		navigationPath(serviceViewName, "Top Pods"):      topPodsRender,
		navigationPath(deploymentViewName, "Config"):     configRender,
		navigationPath(deploymentViewName, "Pods"):       labelsPodsRender,
		navigationPath(deploymentViewName, "Describe"):   describeRender,
		navigationPath(deploymentViewName, "Pods Log"):   podsLogsRender,
		navigationPath(deploymentViewName, "Top Pods"):   topPodsRender,
		navigationPath(podViewName, "Config"):            configRender,
		navigationPath(podViewName, "Log"):               podLogsRender,
		navigationPath(podViewName, "Describe"):          describeRender,
	}
)

func navigationPath(args ...string) string {
	return strings.Join(args, navigationPathJoin)
}

func switchNavigation(index int) string {
	Detail.SetOrigin(0, 0)
	if index < 0 {
		return ""
	}

	if activeView != nil {
		if index >= len(viewNavigationMap[activeView.Name]) {
			return ""
		}
		navigationIndex = index
		activeNavigationOpt = viewNavigationMap[activeView.Name][index]
		return activeNavigationOpt
	}
	return ""
}

func navigationRender(gui *gui.Gui, view *gui.View) error {
	currentView := gui.CurrentView()
	// Change navigation render
	var changeNavigation bool
	if currentView != nil {
		for _, viewName := range functionViews {
			if currentView.Name == viewName {
				if activeView != currentView {
					changeNavigation = true
				}
				activeView = currentView
				break
			}
		}
	}

	if activeView == nil {
		if gui.CurrentView() == nil {
			if err := gui.FocusView(clusterInfoViewName, false); err != nil {
				log.Logger.Println(err)
			}
		}
		activeView = gui.CurrentView()
	}

	options := viewNavigationMap[activeView.Name]
	if activeNavigationOpt == "" {
		activeNavigationOpt = options[navigationIndex]
	}
	if changeNavigation {
		switchNavigation(0)
	}

	colorfulOptions := make([]string, 0)
	for index, opt := range options {
		colorfulOpt := color.White.Sprint(opt)
		if navigationIndex == index {
			colorfulOpt = color.Green.Sprint(opt)
		}
		colorfulOptions = append(colorfulOptions, colorfulOpt)
	}

	view.Clear()
	str := strings.Join(colorfulOptions, OptSeparator)
	fmt.Fprint(view, str)

	return nil
}

func navigationOnClick(gui *gui.Gui, view *gui.View) error {
	cx, cy := view.Cursor()
	log.Logger.Debugf("navigationOnClick - cx %d cy %d", cx, cy)

	options := viewNavigationMap[activeView.Name]
	sep := len(OptSeparator)
	halfSep := sep / 2
	preFix := 0

	var selected string
	for i, opt := range options {
		left := preFix + i*sep

		words := len([]rune(opt))

		right := left + words - 1
		preFix += words - 1

		if cx >= left-halfSep && cx <= right+halfSep {
			log.Logger.Debugf("navigationOnClick - cx %d in selection[%d, %d]", cx, left, right)
			selected = switchNavigation(i)
			break
		}
	}

	log.Logger.Debugf("navigationOnClick - selected '%s'", selected)

	return nil
}

func renderClusterInfo(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	currentContext, err := kubecli.Cli.CurrentContext()
	if err != nil {
		return nil
	}

	if _, err := fmt.Fprintf(view, "Current Context: %s", color.Green.Sprint(currentContext)); err != nil {
		return err
	}
	return nil
}

func detailRender(gui *gui.Gui, view *gui.View) error {
	detailView, err := gui.GetView(detailViewName)
	if err != nil {
		return err
	}

	detailView.Clear()

	if activeView == nil {
		return nil
	}
	renderFunc := detailRenderMap[navigationPath(activeView.Name, activeNavigationOpt)]
	if renderFunc != nil {
		return renderFunc(gui, view)
	}
	return nil
}

func viewStreams(view *gui.View) genericclioptions.IOStreams {
	return genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    view,
		ErrOut: view,
	}
}

func clusterNodesRender(gui *gui.Gui, view *gui.View) error {
	kubecli.Cli.Get(viewStreams(view), "nodes").Run()
	return nil
}

func topNodesRender(gui *gui.Gui, view *gui.View) error {
	kubecli.Cli.TopNode(viewStreams(view), nil, "").Run()
	view.ReRender()
	return nil
}

func namespaceRender(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	streams := newStream()
	kubecli.Cli.Get(streams, "namespaces").Run()
	renderHighlightSelected(view, streamToString(streams))
	return nil
}

func serviceRender(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	streams := newStream()
	if kubecli.Cli.Namespace() == "" {
		kubecli.Cli.Get(streams, "services").SetFlag("all-namespaces", "true").Run()
		renderHighlightSelected(view, streamToString(streams))
		return nil
	}
	kubecli.Cli.Get(streams, "services").Run()
	renderHighlightSelected(view, streamToString(streams))
	return nil
}

func deploymentRender(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	streams := newStream()
	if kubecli.Cli.Namespace() == "" {
		kubecli.Cli.Get(streams, "deployments").SetFlag("all-namespaces", "true").Run()
		renderHighlightSelected(view, streamToString(streams))
		return nil
	}
	kubecli.Cli.Get(viewStreams(view), "deployments").Run()
	renderHighlightSelected(view, streamToString(streams))
	return nil
}

func podRender(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	streams := newStream()
	if kubecli.Cli.Namespace() == "" {
		kubecli.Cli.Get(streams, "pods").SetFlag("all-namespaces", "true").SetFlag("output", "wide").Run()
		renderHighlightSelected(view, streamToString(streams))
		return nil
	}
	kubecli.Cli.Get(streams, "pods").SetFlag("output", "wide").Run()
	renderHighlightSelected(view, streamToString(streams))
	return nil
}

func newStream() genericclioptions.IOStreams {
	return genericclioptions.IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}
}

func streamCopyTo(streams genericclioptions.IOStreams, writer io.Writer) {
	if _, err := io.Copy(writer, (streams.Out).(io.Reader)); err != nil {
		log.Logger.Warningf("streamCopyTo - streams.Out copy error %s", err)
	}
	if _, err := io.Copy(writer, (streams.ErrOut).(io.Reader)); err != nil {
		log.Logger.Warningf("streamCopyTo - streams.ErrOut copy error %s", err)
	}
}

func streamToString(streams genericclioptions.IOStreams) string {
	buf := new(strings.Builder)
	streamCopyTo(streams, buf)
	// check errors
	return buf.String()
}

func renderHighlightSelected(view *gui.View, content string) {
	selected, _ := view.State.Get(selectedViewLine)
	if selected != nil {
		highlightString := selected.(string)
		content = strings.Replace(content, highlightString, color.Green.Sprint(highlightString), 1)
		fmt.Fprint(view, content)
		return
	}
	fmt.Fprint(view, content)
}

func showPleaseSelected(view *gui.View, name string) {
	fmt.Fprintf(view, "Please select a %s. ", name)
}

func namespaceConfigRender(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	namespaceView, err := gui.GetView(namespaceViewName)
	if err != nil {
		return nil
	}
	selected, _ := namespaceView.State.Get(selectedViewLine)
	if selected != nil {
		namespace := formatSelectedNamespace(selected.(string))
		if namespace == "" {
			showPleaseSelected(view, namespaceViewName)
			return nil
		}

		kubecli.Cli.Get(viewStreams(view), "namespaces", namespace).SetFlag("output", "yaml").Run()
		return nil
	}

	showPleaseSelected(view, namespaceViewName)
	return nil
}

func configRender(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	if activeView == nil {
		return nil
	}
	if activeView == Namespace {
		return namespaceConfigRender(gui, view)
	}
	namespaceView, err := gui.GetView(namespaceViewName)
	if err != nil {
		return nil
	}

	selectedNamespace, _ := namespaceView.State.Get(selectedViewLine)
	selected, _ := activeView.State.Get(selectedViewLine)
	resource := ""
	switch activeView.Name {
	case serviceViewName:
		resource = "service"
		break
	case deploymentViewName:
		resource = "deployment"
		break
	case podViewName:
		resource = "pod"
		break
	}

	if resource == "" {
		return nil
	}

	if selected == nil {
		showPleaseSelected(view, resource)
		return nil
	}

	if selectedNamespace != nil {
		selectedName := formatSelectedName(selected.(string), 0)
		if selectedName == "" {
			showPleaseSelected(view, resource)
			return nil
		}
		kubecli.Cli.Get(viewStreams(view), resource, selectedName).SetFlag("output", "yaml").Run()
		return nil
	}

	namespace := formatSelectedName(selected.(string), 0)
	selectedName := formatSelectedName(selected.(string), 1)
	if selectedName == "" {
		showPleaseSelected(view, resource)
		return nil
	}

	kubecli.Cli.WithNamespace(namespace).Get(viewStreams(view), resource, selectedName).SetFlag("output", "yaml").Run()
	return nil
}

func describeRender(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	if activeView == nil {
		return nil
	}
	if activeView == Namespace {
		return namespaceConfigRender(gui, view)
	}
	namespaceView, err := gui.GetView(namespaceViewName)
	if err != nil {
		return nil
	}

	selectedNamespace, _ := namespaceView.State.Get(selectedViewLine)
	selected, _ := activeView.State.Get(selectedViewLine)
	resource := ""
	switch activeView.Name {
	case deploymentViewName:
		resource = "deployment"
		break
	case podViewName:
		resource = "pod"
		break
	}

	if resource == "" {
		return nil
	}

	if selected == nil {
		showPleaseSelected(view, resource)
		return nil
	}

	if selectedNamespace != nil {
		selectedName := formatSelectedName(selected.(string), 0)
		if selectedName == "" {
			showPleaseSelected(view, resource)
			return nil
		}
		kubecli.Cli.Describe(viewStreams(view), resource, selectedName).Run()
		return nil
	}

	namespace := formatSelectedName(selected.(string), 0)
	selectedName := formatSelectedName(selected.(string), 1)
	if selectedName == "" {
		showPleaseSelected(view, resource)
		return nil
	}

	kubecli.Cli.WithNamespace(namespace).Describe(viewStreams(view), resource, selectedName).Run()
	view.ReRender()
	return nil
}

func onFocusClearSelected(gui *gui.Gui, view *gui.View) error {
	for _, functionViewName := range functionViews {
		if functionViewName == view.Name || functionViewName == namespaceViewName {
			continue
		}
		functionView, err := gui.GetView(functionViewName)
		if err != nil {
			log.Logger.Warningf("onFocusClearSelected - view name %s gui.GetView(\"%s\") error %s", view.Name, functionView, err)
			continue
		}
		functionView.State.Set(selectedViewLine, nil)
	}
	return nil
}

func podLogsRender(gui *gui.Gui, view *gui.View) error {
	selectedNamespace, _ := Namespace.State.Get(selectedViewLine)
	selected, _ := Pod.State.Get(selectedViewLine)
	resource := "pod"
	if selected == nil {
		showPleaseSelected(view, resource)
		return nil
	}

	if selectedNamespace != nil {
		selectedName := formatSelectedName(selected.(string), 0)
		if selectedName == "" {
			showPleaseSelected(view, resource)
			return nil
		}
		kubecli.Cli.Logs(viewStreams(view), selectedName).SetFlag("all-containers", "true").SetFlag("tail", logsTail).SetFlag("prefix", "true").Run()
		view.ReRender()
		return nil
	}

	namespace := formatSelectedName(selected.(string), 0)
	selectedName := formatSelectedName(selected.(string), 1)
	if selectedName == "" {
		showPleaseSelected(view, resource)
		return nil
	}

	kubecli.Cli.WithNamespace(namespace).Logs(viewStreams(view), selectedName).SetFlag("all-containers", "true").SetFlag("tail", logsTail).SetFlag("prefix", "true").Run()
	view.ReRender()
	return nil
}

func podsLogsRender(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	if err := podsSelectorRenderHelper(func(namespace string, labelsArr []string) error {
		cmd := kubecli.Cli.WithNamespace(namespace).Logs(viewStreams(view))
		cmd.SetFlag("selector", strings.Join(labelsArr, ","))
		cmd.SetFlag("all-containers", "true").SetFlag("tail", logsTail).SetFlag("prefix", "true").Run()
		view.ReRender()
		return nil
	})(gui, view); err != nil {
		return err
	}
	return nil
}

func labelsPodsRender(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	if err := podsSelectorRenderHelper(func(namespace string, labelsArr []string) error {
		cmd := kubecli.Cli.WithNamespace(namespace).Get(viewStreams(view), "pods")
		cmd.SetFlag("selector", strings.Join(labelsArr, ","))
		cmd.SetFlag("output", "wide")
		cmd.Run()
		view.ReRender()
		return nil
	})(gui, view); err != nil {
		return err
	}
	return nil
}

func topPodsRender(gui *gui.Gui, view *gui.View) error {
	view.Clear()
	if err := podsSelectorRenderHelper(func(namespace string, labelsArr []string) error {
		cmd := kubecli.Cli.WithNamespace(namespace).TopPod(viewStreams(view), nil)
		cmd.SetFlag("selector", strings.Join(labelsArr, ","))
		cmd.Run()
		view.ReRender()
		return nil
	})(gui, view); err != nil {
		return err
	}
	return nil
}

func podsSelectorRenderHelper(cmdFunc func(namespace string, labelsArr []string) error) func(gui *gui.Gui, view *gui.View) error {
	return func(gui *gui.Gui, view *gui.View) error {
		if activeView == nil {
			return nil
		}
		if activeView == Namespace {
			return namespaceConfigRender(gui, view)
		}
		namespaceView, err := gui.GetView(namespaceViewName)
		if err != nil {
			return nil
		}

		selectedNamespace, _ := namespaceView.State.Get(selectedViewLine)
		selected, _ := activeView.State.Get(selectedViewLine)
		var resource string
		var jsonPath string
		switch activeView.Name {
		case serviceViewName:
			resource = "service"
			jsonPath = "jsonpath='{.spec.selector}'"
			break
		case deploymentViewName:
			resource = "deployment"
			jsonPath = "jsonpath='{.spec.selector.matchLabels}'"
			break
		}

		if resource == "" {
			return nil
		}

		if selected == nil {
			showPleaseSelected(view, resource)
			return nil
		}

		output := newStream()
		var namespace string
		if selectedNamespace != nil {
			selectedName := formatSelectedName(selected.(string), 0)
			if selectedName == "" {
				showPleaseSelected(view, resource)
				return nil
			}
			kubecli.Cli.Get(output, resource, selectedName).SetFlag("output", jsonPath).Run()
		} else {
			namespace = formatSelectedName(selected.(string), 0)
			selectedName := formatSelectedName(selected.(string), 1)
			if selectedName == "" {
				showPleaseSelected(view, resource)
				return nil
			}
			kubecli.Cli.WithNamespace(namespace).Get(output, resource, selectedName).SetFlag("output", jsonPath).Run()
		}

		labelJson := streamToString(output)
		if labelJson == "" {
			fmt.Fprint(view, "Pods not found.")
			return nil
		}
		labelsArr := utils.LabelsToStringArr(labelJson[1 : len(labelJson)-1])
		if len(labelsArr) == 0 {
			showPleaseSelected(view, resource)
			return nil
		}

		if namespace == "" {
			namespace = kubecli.Cli.Namespace()
		}

		if err := cmdFunc(namespace, labelsArr); err != nil {
			return err
		}
		return nil
	}
}
