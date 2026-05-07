package cli

import "os"

func osLookupEnv(k string) string { return os.Getenv(k) }
