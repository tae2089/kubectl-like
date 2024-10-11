package kubernetes

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

const (
	logsUsageStr          = "like [-f] [-p] (POD | TYPE/NAME) [-c CONTAINER]"
	defaultPodLogsTimeout = 20 * time.Second
)

var (
	selectorTail    int64 = 10
	logsUsageErrStr       = fmt.Sprintf("expected '%s'.\nPOD or TYPE/NAME is a required argument for the logs command", logsUsageStr)
)

type LikeOptions struct {
	Pattern                        string
	LogOptions                     *logs.LogsOptions
	KubernetesConfigFlags          *genericclioptions.ConfigFlags
	factory                        cmdutil.Factory
	containerNameFromRefSpecRegexp *regexp.Regexp
}

func NewLikeOptions(streams genericiooptions.IOStreams) LikeOptions {
	l := logs.NewLogsOptions(streams)
	KubernetesConfigFlags := genericclioptions.NewConfigFlags(true)
	f := cmdutil.NewFactory(KubernetesConfigFlags)

	return LikeOptions{
		LogOptions:                     l,
		KubernetesConfigFlags:          KubernetesConfigFlags,
		factory:                        f,
		containerNameFromRefSpecRegexp: regexp.MustCompile(`spec\.(?:initContainers|containers|ephemeralContainers){(.+)}`),
	}
}

func (l *LikeOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&l.LogOptions.AllPods, "all-pods", l.LogOptions.AllPods, "Get logs from all pod(s). Sets prefix to true.")
	cmd.Flags().BoolVar(&l.LogOptions.AllContainers, "all-containers", l.LogOptions.AllContainers, "Get all containers' logs in the pod(s).")
	cmd.Flags().BoolVarP(&l.LogOptions.Follow, "follow", "f", l.LogOptions.Follow, "Specify if the logs should be streamed.")
	cmd.Flags().BoolVar(&l.LogOptions.Timestamps, "timestamps", l.LogOptions.Timestamps, "Include timestamps on each line in the log output")
	cmd.Flags().Int64Var(&l.LogOptions.LimitBytes, "limit-bytes", l.LogOptions.LimitBytes, "Maximum bytes of logs to return. Defaults to no limit.")
	cmd.Flags().BoolVarP(&l.LogOptions.Previous, "previous", "p", l.LogOptions.Previous, "If true, print the logs for the previous instance of the container in a pod if it exists.")
	cmd.Flags().Int64Var(&l.LogOptions.Tail, "tail", l.LogOptions.Tail, "Lines of recent log file to display. Defaults to -1 with no selector, showing all log lines otherwise 10, if a selector is provided.")
	cmd.Flags().BoolVar(&l.LogOptions.IgnoreLogErrors, "ignore-errors", l.LogOptions.IgnoreLogErrors, "If watching / following pod logs, allow for any errors that occur to be non-fatal")
	cmd.Flags().StringVar(&l.LogOptions.SinceTime, "since-time", l.LogOptions.SinceTime, i18n.T("Only return logs after a specific date (RFC3339). Defaults to all logs. Only one of since-time / since may be used."))
	cmd.Flags().DurationVar(&l.LogOptions.SinceSeconds, "since", l.LogOptions.SinceSeconds, "Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs. Only one of since-time / since may be used.")
	cmd.Flags().StringVarP(&l.LogOptions.Container, "container", "c", l.LogOptions.Container, "Print the logs of this container")
	cmd.Flags().BoolVar(&l.LogOptions.InsecureSkipTLSVerifyBackend, "insecure-skip-tls-verify-backend", l.LogOptions.InsecureSkipTLSVerifyBackend,
		"Skip verifying the identity of the kubelet that logs are requested from.  In theory, an attacker could provide invalid log content back. You might want to use this if your kubelet serving certificates have expired.")
	cmdutil.AddPodRunningTimeoutFlag(cmd, defaultPodLogsTimeout)
	cmdutil.AddLabelSelectorFlagVar(cmd, &l.LogOptions.Selector)
	cmd.Flags().IntVar(&l.LogOptions.MaxFollowConcurrency, "max-log-requests", l.LogOptions.MaxFollowConcurrency, "Specify maximum number of concurrent logs to follow when using by a selector. Defaults to 5.")
	cmd.Flags().BoolVar(&l.LogOptions.Prefix, "prefix", l.LogOptions.Prefix, "Prefix each log line with the log source (pod name and container name)")
	cmd.Flags().StringVar(&l.Pattern, "pattern", "", "If true, print the logs for the previous instance of the container in a pod if it exists.")
	l.KubernetesConfigFlags.AddFlags(cmd.Flags())

	l.hiddenGlobalFlags(cmd)

	filters := []string{"options"}
	templates.ActsAsRootCommand(cmd, filters)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
}

func (l *LikeOptions) hiddenGlobalFlags(cmd *cobra.Command) {
	hiddentFlags := []string{"namespace", "server", "kubeconfig", "context", "cluster", "as", "as-group", "as-uid", "cache-dir", "certificate-authority", "client-certificate", "client-key", "disable-compression", "insecure-skip-tls-verify", "request-timeout", "tls-server-name", "token", "user"}
	for _, flag := range hiddentFlags {
		cmd.Flags().MarkHidden(flag)
	}
}

func (l *LikeOptions) ToLikeOptions() (*corev1.PodLogOptions, error) {
	logOptions := &corev1.PodLogOptions{
		Container:                    l.LogOptions.Container,
		Follow:                       l.LogOptions.Follow,
		Previous:                     l.LogOptions.Previous,
		Timestamps:                   l.LogOptions.Timestamps,
		InsecureSkipTLSVerifyBackend: l.LogOptions.InsecureSkipTLSVerifyBackend,
	}

	if len(l.LogOptions.SinceTime) > 0 {
		t, err := util.ParseRFC3339(l.LogOptions.SinceTime, metav1.Now)
		if err != nil {
			return nil, err
		}

		logOptions.SinceTime = &t
	}

	if l.LogOptions.LimitBytes != 0 {
		logOptions.LimitBytes = &l.LogOptions.LimitBytes
	}

	if l.LogOptions.SinceSeconds != 0 {
		// round up to the nearest second
		sec := int64(l.LogOptions.SinceSeconds.Round(time.Second).Seconds())
		logOptions.SinceSeconds = &sec
	}

	if len(l.LogOptions.Selector) > 0 && l.LogOptions.Tail == -1 && !l.LogOptions.TailSpecified {
		logOptions.TailLines = &selectorTail
	} else if l.LogOptions.Tail != -1 {
		logOptions.TailLines = &l.LogOptions.Tail
	}

	return logOptions, nil
}

func (l *LikeOptions) Complete(args []string, cmd *cobra.Command) error {

	l.LogOptions.ContainerNameSpecified = cmd.Flag("container").Changed
	l.LogOptions.TailSpecified = cmd.Flag("tail").Changed
	l.LogOptions.Resources = args

	switch len(args) {
	case 0:
		if len(l.LogOptions.Selector) == 0 {
			return cmdutil.UsageErrorf(cmd, "%s", logsUsageErrStr)
		}
	case 1:
		l.LogOptions.ResourceArg = args[0]
		if len(l.LogOptions.Selector) != 0 {
			return cmdutil.UsageErrorf(cmd, "only a selector (-l) or a POD name is allowed")
		}
	case 2:
		l.LogOptions.ResourceArg = args[0]
		l.LogOptions.Container = args[1]
	default:
		return cmdutil.UsageErrorf(cmd, "%s", logsUsageErrStr)
	}

	if l.LogOptions.AllPods {
		l.LogOptions.Prefix = true
	}

	var err error
	namespace, _, err := l.KubernetesConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	l.LogOptions.Namespace = namespace
	l.LogOptions.ConsumeRequestFn = l.DefaultConsumeRequest
	l.LogOptions.GetPodTimeout, err = cmdutil.GetPodRunningTimeoutFlag(cmd)
	if err != nil {
		return err
	}

	l.LogOptions.Options, err = l.LogOptions.ToLogOptions()
	if err != nil {
		return err
	}

	l.LogOptions.RESTClientGetter = l.factory
	l.LogOptions.LogsForObject = polymorphichelpers.LogsForObjectFn
	l.LogOptions.AllPodLogsForObject = polymorphichelpers.AllPodLogsForObjectFn

	if l.LogOptions.Object == nil {
		builder := l.factory.NewBuilder().
			WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
			NamespaceParam(l.LogOptions.Namespace).DefaultNamespace().
			SingleResourceType()
		if l.LogOptions.ResourceArg != "" {
			builder.ResourceNames("pods", l.LogOptions.ResourceArg)
		}
		if l.LogOptions.Selector != "" {
			builder.ResourceTypes("pods").LabelSelectorParam(l.LogOptions.Selector)
		}
		infos, err := builder.Do().Infos()
		if err != nil {
			if apierrors.IsNotFound(err) {
				err = fmt.Errorf("error from server (NotFound): %w in namespace %q", err, l.LogOptions.Namespace)
			}
			return err
		}
		if l.LogOptions.Selector == "" && len(infos) != 1 {
			return errors.New("expected a resource")
		}
		l.LogOptions.Object = infos[0].Object
		if l.LogOptions.Selector != "" && len(l.LogOptions.Object.(*corev1.PodList).Items) == 0 {
			fmt.Fprintf(l.LogOptions.ErrOut, "No resources found in %s namespace.\n", l.LogOptions.Namespace)
		}
	}

	return nil
}

func (l LikeOptions) Vaildate() error {

	if l.Pattern == "" {
		return fmt.Errorf("pattern is required. Please provide a pattern to match the logs")
	}

	if len(l.LogOptions.SinceTime) > 0 && l.LogOptions.SinceSeconds != 0 {
		return fmt.Errorf("at most one of `sinceTime` or `sinceSeconds` may be specified")
	}

	logsOptions, ok := l.LogOptions.Options.(*corev1.PodLogOptions)
	if !ok {
		return errors.New("unexpected logs options object")
	}
	if l.LogOptions.AllContainers && len(logsOptions.Container) > 0 {
		return fmt.Errorf("--all-containers=true should not be specified with container name %s", logsOptions.Container)
	}

	if l.LogOptions.ContainerNameSpecified && len(l.LogOptions.Resources) == 2 {
		return fmt.Errorf("only one of -c or an inline [CONTAINER] arg is allowed")
	}

	if l.LogOptions.LimitBytes < 0 {
		return fmt.Errorf("--limit-bytes must be greater than 0")
	}

	if logsOptions.SinceSeconds != nil && *logsOptions.SinceSeconds < int64(0) {
		return fmt.Errorf("--since must be greater than 0")
	}

	if logsOptions.TailLines != nil && *logsOptions.TailLines < -1 {
		return fmt.Errorf("--tail must be greater than or equal to -1")
	}
	return nil
}

func (l LikeOptions) Run() error {
	var requests map[corev1.ObjectReference]rest.ResponseWrapper
	var err error
	if l.LogOptions.AllPods {
		requests, err = l.LogOptions.AllPodLogsForObject(l.LogOptions.RESTClientGetter, l.LogOptions.Object, l.LogOptions.Options, l.LogOptions.GetPodTimeout, l.LogOptions.AllContainers)
	} else {
		requests, err = l.LogOptions.LogsForObject(l.LogOptions.RESTClientGetter, l.LogOptions.Object, l.LogOptions.Options, l.LogOptions.GetPodTimeout, l.LogOptions.AllContainers)
	}
	if err != nil {
		return err
	}

	if l.LogOptions.Follow && len(requests) > 1 {
		if len(requests) > l.LogOptions.MaxFollowConcurrency {
			return fmt.Errorf(
				"you are attempting to follow %d log streams, but maximum allowed concurrency is %d, use --max-log-requests to increase the limit",
				len(requests), l.LogOptions.MaxFollowConcurrency,
			)
		}

		return l.parallelConsumeRequest(requests)
	}

	return l.sequentialConsumeRequest(requests)
}

func (l LikeOptions) parallelConsumeRequest(requests map[corev1.ObjectReference]rest.ResponseWrapper) error {
	reader, writer := io.Pipe()
	wg := &sync.WaitGroup{}
	wg.Add(len(requests))
	for objRef, request := range requests {
		go func(objRef corev1.ObjectReference, request rest.ResponseWrapper) {
			defer wg.Done()
			out := l.addPrefixIfNeeded(objRef, writer)
			if err := l.LogOptions.ConsumeRequestFn(request, out); err != nil {
				if !l.LogOptions.IgnoreLogErrors {
					writer.CloseWithError(err)

					// It's important to return here to propagate the error via the pipe
					return
				}

				fmt.Fprintf(writer, "error: %v\n", err)
			}

		}(objRef, request)
	}

	go func() {
		wg.Wait()
		writer.Close()
	}()

	_, err := io.Copy(l.LogOptions.Out, reader)
	return err
}

func (l LikeOptions) sequentialConsumeRequest(requests map[corev1.ObjectReference]rest.ResponseWrapper) error {
	for objRef, request := range requests {
		out := l.addPrefixIfNeeded(objRef, l.LogOptions.Out)
		if err := l.LogOptions.ConsumeRequestFn(request, out); err != nil {
			if !l.LogOptions.IgnoreLogErrors {
				return err
			}

			fmt.Fprintf(l.LogOptions.Out, "error: %v\n", err)
		}
	}

	return nil
}

func (l LikeOptions) addPrefixIfNeeded(ref corev1.ObjectReference, writer io.Writer) io.Writer {
	if !l.LogOptions.Prefix || ref.FieldPath == "" || ref.Name == "" {
		return writer
	}

	// We rely on ref.FieldPath to contain a reference to a container
	// including a container name (not an index) so we can get a container name
	// without making an extra API request.
	var containerName string
	containerNameMatches := l.containerNameFromRefSpecRegexp.FindStringSubmatch(ref.FieldPath)
	if len(containerNameMatches) == 2 {
		containerName = containerNameMatches[1]
	}

	prefix := fmt.Sprintf("[pod/%s/%s] ", ref.Name, containerName)
	return &prefixingWriter{
		prefix: []byte(prefix),
		writer: writer,
	}
}

func (l LikeOptions) DefaultConsumeRequest(request rest.ResponseWrapper, out io.Writer) error {
	readCloser, err := request.Stream(context.TODO())
	if err != nil {
		return err
	}
	defer readCloser.Close()
	// Compile the regular expression
	re, err := regexp.Compile(l.Pattern)
	if err != nil {
		return err
	}

	r := bufio.NewReader(readCloser)
	for {
		bytes, err := r.ReadBytes('\n')
		if re.Match(bytes) {
			if _, err := out.Write(bytes); err != nil {
				return err
			}
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}

type prefixingWriter struct {
	prefix []byte
	writer io.Writer
}

func (pw *prefixingWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	// Perform an "atomic" write of a prefix and p to make sure that it doesn't interleave
	// sub-line when used concurrently with io.PipeWrite.
	n, err := pw.writer.Write(append(pw.prefix, p...))
	if n > len(p) {
		// To comply with the io.Writer interface requirements we must
		// return a number of bytes written from p (0 <= n <= len(p)),
		// so we are ignoring the length of the prefix here.
		return len(p), err
	}
	return n, err
}
