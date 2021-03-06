package commands

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/errors"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/server"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/stats"
	"github.com/argoproj/argo-cd/util/tls"
)

const (
	// DefaultDexServerAddr is the HTTP address of the Dex OIDC server, which we run a reverse proxy against
	DefaultDexServerAddr = "http://dex-server:5556"

	// DefaultRepoServerAddr is the gRPC address of the Argo CD repo server
	DefaultRepoServerAddr = "argocd-repo-server:8081"
)

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		insecure               bool
		logLevel               string
		glogLevel              int
		clientConfig           clientcmd.ClientConfig
		staticAssetsDir        string
		repoServerAddress      string
		dexServerAddress       string
		disableAuth            bool
		tlsConfigCustomizerSrc func() (tls.ConfigCustomizer, error)
	)
	var command = &cobra.Command{
		Use:   cliName,
		Short: "Run the argocd API server",
		Long:  "Run the argocd API server",
		Run: func(c *cobra.Command, args []string) {
			cli.SetLogLevel(logLevel)
			cli.SetGLogLevel(glogLevel)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			tlsConfigCustomizer, err := tlsConfigCustomizerSrc()
			errors.CheckError(err)

			kubeclientset := kubernetes.NewForConfigOrDie(config)
			appclientset := appclientset.NewForConfigOrDie(config)
			repoclientset := reposerver.NewRepositoryServerClientset(repoServerAddress)

			argoCDOpts := server.ArgoCDServerOpts{
				Insecure:            insecure,
				Namespace:           namespace,
				StaticAssetsDir:     staticAssetsDir,
				KubeClientset:       kubeclientset,
				AppClientset:        appclientset,
				RepoClientset:       repoclientset,
				DexServerAddr:       dexServerAddress,
				DisableAuth:         disableAuth,
				TLSConfigCustomizer: tlsConfigCustomizer,
			}

			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")

			for {
				argocd := server.NewServer(argoCDOpts)
				ctx := context.Background()
				ctx, cancel := context.WithCancel(ctx)
				argocd.Run(ctx, 8080)
				cancel()
			}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().BoolVar(&insecure, "insecure", false, "Run server without TLS")
	command.Flags().StringVar(&staticAssetsDir, "staticassets", "", "Static assets directory path")
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	command.Flags().StringVar(&repoServerAddress, "repo-server", DefaultRepoServerAddr, "Repo server address")
	command.Flags().StringVar(&dexServerAddress, "dex-server", DefaultDexServerAddr, "Dex server address")
	command.Flags().BoolVar(&disableAuth, "disable-auth", false, "Disable client authentication")
	command.AddCommand(cli.NewVersionCmd(cliName))
	tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(command)
	return command
}
