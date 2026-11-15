package config

import (
	"os"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestCacheEnvOverride(t *testing.T) {
	c := &Config{Cache: Cache{Enabled: false}}
	t.Setenv("BITE_CACHE_ENABLED", "true")
	applyEnvOverrides(c)
	if !c.Cache.Enabled {
		t.Fatal("BITE_CACHE_ENABLED=true should enable the cache")
	}
	t.Setenv("BITE_CACHE_ENABLED", "0")
	applyEnvOverrides(c)
	if c.Cache.Enabled {
		t.Fatal("BITE_CACHE_ENABLED=0 should disable the cache")
	}
	t.Setenv("BITE_CACHE_ENABLED", "maybe")
	applyEnvOverrides(c)
	if c.Cache.Enabled {
		t.Fatal("an unparseable value must be ignored")
	}
	t.Setenv("BITE_CACHE_ENABLED", "")
	c.Cache.Enabled = true
	applyEnvOverrides(c)
	if !c.Cache.Enabled {
		t.Fatal("unset leaves the file's value alone")
	}
}

// The YAML files we ship must decode strictly (a mistyped key fails instead of being silently
// ignored) and carry the cache settings.
func TestShippedConfigs(t *testing.T) {
	for _, file := range []string{"../../config-local.yml", "../../config-docker.yml"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var c Config
		if err := yaml.UnmarshalStrict(raw, &c); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if !c.Cache.Enabled || c.Cache.Type != "memory" || c.Cache.Config.Ttl <= 0 || c.Cache.Config.MaxEntries <= 0 {
			t.Errorf("%s: cache not configured: %+v", file, c.Cache)
		}
	}
}
