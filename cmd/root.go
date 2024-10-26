package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	kube "github.com/tae2089/kubectl-like/pkg/kubernetes"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func CreateRootCmd() *cobra.Command {
	ioStreams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	l := kube.NewLikeOptions(ioStreams)
	rootCmd := &cobra.Command{
		Use:                   "kubectl like [-f] [-p] (POD | TYPE/NAME) --pattern [-c CONTAINER] [options]",
		Short:                 "logging pods using regex pattern",
		Long:                  "logging pods using regex pattern",
		DisableFlagsInUseLine: true,
		SilenceErrors:         true,
		SilenceUsage:          true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			cmdutil.CheckErr(l.Complete(args, cmd))
			cmdutil.CheckErr(l.Vaildate())
			cmdutil.CheckErr(l.Run())
			return nil
		},
	}
	l.AddFlags(rootCmd)
	l.RegisterCompletionFunc(rootCmd)
	return rootCmd
}
