package itest

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestScenarios globs scenarios/*.yaml and runs each as a named subtest
// against a fresh flspd process + Postgres container on the shared devnet
// (StartDevnet is idempotent -- see devnet.go). This is T4-1's generic
// scenario runner: the scenario files themselves (T4-2) describe the
// steps/assertions declaratively; this file only wires a scenario's step
// sequence to a live ScenarioContext and reports failures per scenario.
func TestScenarios(t *testing.T) {
	files, err := filepath.Glob("scenarios/*.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, files, "no scenario files found under scenarios/")

	d := StartDevnet(t)

	for _, path := range files {
		path := path
		scenario, err := loadScenario(path)
		require.NoError(t, err, "loading %s", path)

		t.Run(scenario.Name, func(t *testing.T) {
			ctx := t.Context()

			pg, err := NewPostgresContainer(t.TempDir() + "/postgres.log")
			require.NoError(t, err)
			require.NoError(t, pg.Start(ctx))
			t.Cleanup(func() { pg.Stop(ctx) })

			flspd := NewFlspdProcess(t, d, pg, FlspdProcessOptions{JitEnabled: true})
			require.NoError(t, flspd.Start())
			t.Cleanup(func() { flspd.Stop() })

			sc := newScenarioContext(t, ctx, d, pg, flspd)

			for i, step := range scenario.Steps {
				fn, ok := stepRegistry[step.Verb]
				require.True(t, ok, "step[%d] %q: unknown verb (should have been caught at load time)", i, step.Verb)
				if err := fn(t, sc, step.Params); err != nil {
					t.Fatalf("step[%d] %s (line %d): %v", i, step.Verb, step.Line, err)
				}
			}

			for i, a := range scenario.Assertions {
				fn, ok := assertRegistry[a.Verb]
				require.True(t, ok, "assertions[%d] %q: unknown verb (should have been caught at load time)", i, a.Verb)
				fn(t, sc, a.Params)
			}
		})
	}
}
