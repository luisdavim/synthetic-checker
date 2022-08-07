package server

import (
	"fmt"
	"strings"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func mustBindPFlag(key string, f *flag.Flag) {
	if err := viper.BindPFlag(key, f); err != nil {
		panic(fmt.Sprintf("viper.BindPFlag(%s) failed: %v", key, err))
	}
}

func init() {
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/config")
	viper.SetConfigName("server")
	viper.SetConfigType("yaml")

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.SetEnvPrefix("http")
	viper.AutomaticEnv()

	// Server flags
	flagSet := flag.NewFlagSet("server", flag.ExitOnError)
	flagSet.BoolP("debug", "d", false, "Set log level to debug")
	flagSet.IntP("port", "p", 8080, "Port for the http listener")
	flagSet.IntP("securePort", "s", 8443, "Port for the HTTPS listener")
	flagSet.StringP("user", "U", "", "Set BasicAuth user for the http listener")
	flagSet.StringP("pass", "P", "", "Set BasicAuth password for the http listener")
	flagSet.StringP("certFile", "C", "", "File containing the x509 Certificate for HTTPS.")
	flagSet.StringP("keyFile", "K", "", "File containing the x509 private key for HTTPS.")
	flagSet.IntP("request-limit", "l", 0, "Max requests per second per client allowed")
	flagSet.BoolP("pretty-json", "", false, "Pretty print JSON responses")
	flagSet.BoolP("strip-slashes", "S", false, "Strip trailing slashes befofore matching routes")

	flag.CommandLine.AddFlagSet(flagSet)
	if err := flagSet.Parse(flag.Args()); err != nil {
		panic(err)
	}

	// viper.BindPFlags(flag.CommandLine)
	mustBindPFlag("debug", flagSet.Lookup("debug"))
	mustBindPFlag("stripSlashes", flagSet.Lookup("strip-slashes"))
	mustBindPFlag("http.port", flagSet.Lookup("port"))
	mustBindPFlag("http.securePort", flagSet.Lookup("securePort"))
	mustBindPFlag("http.auth.user", flagSet.Lookup("user"))
	mustBindPFlag("http.auth.pass", flagSet.Lookup("pass"))
	mustBindPFlag("http.certFile", flagSet.Lookup("certFile"))
	mustBindPFlag("http.keyFile", flagSet.Lookup("keyFile"))
	mustBindPFlag("http.requestLimit", flagSet.Lookup("request-limit"))
	mustBindPFlag("http.prettyJSON", flagSet.Lookup("pretty-json"))
}
