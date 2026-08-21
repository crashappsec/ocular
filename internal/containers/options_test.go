// Copyright (C) 2025-2026 Crash Override, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the FSF, either version 3 of the License, or (at your option) any later version.
// See the LICENSE file in the root of this repository for full license text or
// visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

package containers

import (
	"reflect"
	"testing"

	"github.com/crashappsec/ocular/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

/* helpers */

func envNames(envs []corev1.EnvVar) []string {
	names := make([]string, 0, len(envs))
	for _, e := range envs {
		names = append(names, e.Name)
	}
	return names
}

func lookupEnv(t *testing.T, envs []corev1.EnvVar, name string) corev1.EnvVar {
	t.Helper()
	for _, e := range envs {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("env var %q not found, have %v", name, envNames(envs))
	return corev1.EnvVar{}
}

// recordingOption returns an Option that appends its label to *log when run,
// so tests can assert on application order.
func recordingOption(log *[]string, label string) Option {
	return func(c *corev1.Container) {
		*log = append(*log, label)
	}
}

/* tests */

func TestWithAdditionalEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		existing []corev1.EnvVar
		add      []corev1.EnvVar
		want     []string
	}{
		{
			name: "appends to empty",
			add:  []corev1.EnvVar{{Name: "A", Value: "1"}},
			want: []string{"A"},
		},
		{
			name:     "appends after existing, preserving order",
			existing: []corev1.EnvVar{{Name: "A", Value: "1"}},
			add:      []corev1.EnvVar{{Name: "B", Value: "2"}, {Name: "C", Value: "3"}},
			want:     []string{"A", "B", "C"},
		},
		{
			name:     "no vars is a no-op",
			existing: []corev1.EnvVar{{Name: "A", Value: "1"}},
			add:      nil,
			want:     []string{"A"},
		},
		{
			name:     "duplicates are not de-duplicated",
			existing: []corev1.EnvVar{{Name: "A", Value: "1"}},
			add:      []corev1.EnvVar{{Name: "A", Value: "2"}},
			want:     []string{"A", "A"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := corev1.Container{Env: tt.existing}
			WithAdditionalEnvVars(tt.add...)(&c)

			if got := envNames(c.Env); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("env names = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithAdditionalEnvVarsPreservesValues(t *testing.T) {
	c := corev1.Container{}
	want := corev1.EnvVar{
		Name: "FROM_FIELD",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
		},
	}

	WithAdditionalEnvVars(want)(&c)

	if got := lookupEnv(t, c.Env, "FROM_FIELD"); !reflect.DeepEqual(got, want) {
		t.Errorf("env var = %+v, want %+v", got, want)
	}
}

func TestWithAdditionalArgs(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		add      []string
		want     []string
	}{
		{name: "appends to empty", add: []string{"--flag"}, want: []string{"--flag"}},
		{
			name:     "appends after existing",
			existing: []string{"--first"},
			add:      []string{"--second", "--third"},
			want:     []string{"--first", "--second", "--third"},
		},
		{
			name:     "no args is a no-op",
			existing: []string{"--first"},
			want:     []string{"--first"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := corev1.Container{Args: tt.existing}
			WithAdditionalArgs(tt.add...)(&c)

			if !reflect.DeepEqual(c.Args, tt.want) {
				t.Errorf("Args = %v, want %v", c.Args, tt.want)
			}
		})
	}
}

func TestWithAdditionalArgsDoesNotTouchCommand(t *testing.T) {
	c := corev1.Container{Command: []string{"/bin/sh"}, Args: []string{"-c"}}

	WithAdditionalArgs("echo hi")(&c)

	if want := []string{"/bin/sh"}; !reflect.DeepEqual(c.Command, want) {
		t.Errorf("Command = %v, want %v", c.Command, want)
	}
}

func TestWithNamePrefix(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		prefix   string
		want     string
	}{
		{name: "prepends", existing: "scanner", prefix: "pre-", want: "pre-scanner"},
		{name: "empty prefix is a no-op", existing: "scanner", want: "scanner"},
		{name: "empty name yields bare prefix", existing: "", prefix: "pre-", want: "pre-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := corev1.Container{Name: tt.existing}
			WithNamePrefix(tt.prefix)(&c)

			if c.Name != tt.want {
				t.Errorf("Name = %q, want %q", c.Name, tt.want)
			}
		})
	}
}

func TestWithNamePrefixAppliedTwiceNestsOutward(t *testing.T) {
	c := corev1.Container{Name: "scanner"}

	c = ApplyOptionsTo(c, WithNamePrefix("inner-"), WithNamePrefix("outer-"))

	if want := "outer-inner-scanner"; c.Name != want {
		t.Errorf("Name = %q, want %q", c.Name, want)
	}
}

func TestWrapCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     []string
		args        []string
		entrypoint  []string
		wantCommand []string
		wantArgs    []string
	}{
		{
			name:        "old command is prepended to args",
			command:     []string{"/bin/scanner"},
			args:        []string{"--target", "."},
			entrypoint:  []string{"/wrapper"},
			wantCommand: []string{"/wrapper"},
			wantArgs:    []string{"/bin/scanner", "--target", "."},
		},
		{
			name:        "no existing command leaves args untouched",
			args:        []string{"--target", "."},
			entrypoint:  []string{"/wrapper"},
			wantCommand: []string{"/wrapper"},
			wantArgs:    []string{"--target", "."},
		},
		{
			name:        "no existing args keeps only the old command",
			command:     []string{"/bin/scanner"},
			entrypoint:  []string{"/wrapper", "--exec"},
			wantCommand: []string{"/wrapper", "--exec"},
			wantArgs:    []string{"/bin/scanner"},
		},
		{
			name:        "multi-element command is preserved in order",
			command:     []string{"/bin/sh", "-c"},
			args:        []string{"echo hi"},
			entrypoint:  []string{"/wrapper"},
			wantCommand: []string{"/wrapper"},
			wantArgs:    []string{"/bin/sh", "-c", "echo hi"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := corev1.Container{Command: tt.command, Args: tt.args}
			WrapCommand(tt.entrypoint...)(&c)

			if !reflect.DeepEqual(c.Command, tt.wantCommand) {
				t.Errorf("Command = %v, want %v", c.Command, tt.wantCommand)
			}
			if !reflect.DeepEqual(c.Args, tt.wantArgs) {
				t.Errorf("Args = %v, want %v", c.Args, tt.wantArgs)
			}
		})
	}
}

func TestWrapCommandWithNoEntrypointClearsCommand(t *testing.T) {
	c := corev1.Container{Command: []string{"/bin/scanner"}, Args: []string{"--target"}}

	WrapCommand()(&c)

	if len(c.Command) != 0 {
		t.Errorf("Command = %v, want empty", c.Command)
	}
	if want := []string{"/bin/scanner", "--target"}; !reflect.DeepEqual(c.Args, want) {
		t.Errorf("Args = %v, want %v", c.Args, want)
	}
}

func TestWrapCommandDoesNotAliasCallerBackingArray(t *testing.T) {
	backing := []string{"/bin/scanner", "sentinel"}
	c := corev1.Container{
		Command: backing[:1], // len 1, cap 2
		Args:    []string{"--target"},
	}

	WrapCommand("/wrapper")(&c)

	if backing[1] != "sentinel" {
		t.Errorf("WrapCommand wrote into the caller's backing array: backing[1] = %q, want %q",
			backing[1], "sentinel")
	}
}

func TestWithAdditionalVolumeMounts(t *testing.T) {
	existing := corev1.VolumeMount{Name: "workspace", MountPath: "/workspace"}
	added := corev1.VolumeMount{Name: "results", MountPath: "/results", ReadOnly: true}

	c := corev1.Container{VolumeMounts: []corev1.VolumeMount{existing}}
	WithAdditionalVolumeMounts(added)(&c)

	want := []corev1.VolumeMount{existing, added}
	if !reflect.DeepEqual(c.VolumeMounts, want) {
		t.Errorf("VolumeMounts = %+v, want %+v", c.VolumeMounts, want)
	}
}

func TestWithAdditionalVolumeMountsEmptyIsNoOp(t *testing.T) {
	existing := []corev1.VolumeMount{{Name: "workspace", MountPath: "/workspace"}}
	c := corev1.Container{VolumeMounts: existing}

	WithAdditionalVolumeMounts()(&c)

	if !reflect.DeepEqual(c.VolumeMounts, existing) {
		t.Errorf("VolumeMounts = %+v, want %+v", c.VolumeMounts, existing)
	}
}

func TestWithWorkingDir(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		dir      string
		want     string
	}{
		{name: "sets when unset", dir: "/workspace", want: "/workspace"},
		{name: "overwrites existing", existing: "/old", dir: "/new", want: "/new"},
		{name: "empty string clears it", existing: "/old", dir: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := corev1.Container{WorkingDir: tt.existing}
			WithWorkingDir(tt.dir)(&c)

			if c.WorkingDir != tt.want {
				t.Errorf("WorkingDir = %q, want %q", c.WorkingDir, tt.want)
			}
		})
	}
}

func TestApplyOptionsToNoOptionsReturnsEqualContainer(t *testing.T) {
	in := corev1.Container{Name: "scanner", Image: "example/scanner:latest"}

	out := ApplyOptionsTo(in)

	if !reflect.DeepEqual(out, in) {
		t.Errorf("container = %+v, want %+v", out, in)
	}
}

func TestApplyOptionsToAppliesInOrder(t *testing.T) {
	var log []string

	ApplyOptionsTo(corev1.Container{},
		recordingOption(&log, "first"),
		recordingOption(&log, "second"),
		recordingOption(&log, "third"),
	)

	if want := []string{"first", "second", "third"}; !reflect.DeepEqual(log, want) {
		t.Errorf("application order = %v, want %v", log, want)
	}
}

func TestApplyOptionsToDoesNotMutateInputScalars(t *testing.T) {
	in := corev1.Container{Name: "scanner", WorkingDir: "/original"}

	out := ApplyOptionsTo(in, WithNamePrefix("pre-"), WithWorkingDir("/changed"))

	if in.Name != "scanner" {
		t.Errorf("input Name was mutated: got %q, want %q", in.Name, "scanner")
	}
	if in.WorkingDir != "/original" {
		t.Errorf("input WorkingDir was mutated: got %q, want %q", in.WorkingDir, "/original")
	}
	if out.Name != "pre-scanner" || out.WorkingDir != "/changed" {
		t.Errorf("returned container = %q %q, want %q %q",
			out.Name, out.WorkingDir, "pre-scanner", "/changed")
	}
}

func TestApplyOptionsToCombinesMultipleOptions(t *testing.T) {
	c := ApplyOptionsTo(corev1.Container{Name: "scanner"},
		WithNamePrefix("pre-"),
		WithWorkingDir("/workspace"),
		WithAdditionalArgs("--verbose"),
		WithAdditionalEnvVars(corev1.EnvVar{Name: "LOG_LEVEL", Value: "debug"}),
	)

	if c.Name != "pre-scanner" {
		t.Errorf("Name = %q, want %q", c.Name, "pre-scanner")
	}
	if c.WorkingDir != "/workspace" {
		t.Errorf("WorkingDir = %q, want %q", c.WorkingDir, "/workspace")
	}
	if want := []string{"--verbose"}; !reflect.DeepEqual(c.Args, want) {
		t.Errorf("Args = %v, want %v", c.Args, want)
	}
	if got := lookupEnv(t, c.Env, "LOG_LEVEL"); got.Value != "debug" {
		t.Errorf("LOG_LEVEL = %q, want %q", got.Value, "debug")
	}
}

func TestApplyOptionsToAllAppliesToEveryContainer(t *testing.T) {
	in := []corev1.Container{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	out := ApplyOptionsToAll(in, WithNamePrefix("pre-"))

	want := []string{"pre-a", "pre-b", "pre-c"}
	got := make([]string, 0, len(out))
	for _, c := range out {
		got = append(got, c.Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

func TestApplyOptionsToAllAppliesAllOptions(t *testing.T) {
	out := ApplyOptionsToAll(
		[]corev1.Container{{Name: "a"}, {Name: "b"}},
		WithNamePrefix("pre-"),
		WithWorkingDir("/workspace"),
		WithAdditionalArgs("--verbose"),
	)

	for _, c := range out {
		if c.WorkingDir != "/workspace" {
			t.Errorf("container %q: WorkingDir = %q, want %q", c.Name, c.WorkingDir, "/workspace")
		}
		if want := []string{"--verbose"}; !reflect.DeepEqual(c.Args, want) {
			t.Errorf("container %q: Args = %v, want %v", c.Name, c.Args, want)
		}
	}
}

// ApplyOptionsToAll takes a slice, so it mutates the caller's containers in
// place. This test pins that behaviour so a future change to value semantics
// is a deliberate decision rather than an accident.
func TestApplyOptionsToAllMutatesInputSliceInPlace(t *testing.T) {
	in := []corev1.Container{{Name: "a"}}

	ApplyOptionsToAll(in, WithNamePrefix("pre-"))

	if in[0].Name != "pre-a" {
		t.Errorf("input slice element = %q, want %q", in[0].Name, "pre-a")
	}
}

func TestApplyOptionsToAllEmptyInputs(t *testing.T) {
	t.Run("nil containers", func(t *testing.T) {
		if out := ApplyOptionsToAll(nil, WithNamePrefix("pre-")); len(out) != 0 {
			t.Errorf("got %d containers, want 0", len(out))
		}
	})

	t.Run("no options leaves containers untouched", func(t *testing.T) {
		in := []corev1.Container{{Name: "a", Image: "img"}}
		out := ApplyOptionsToAll(in)

		if !reflect.DeepEqual(out, in) {
			t.Errorf("containers = %+v, want %+v", out, in)
		}
	})
}

func TestApplyOptionsToAllConditionalReturnsOneContainerPerInput(t *testing.T) {
	in := []v1beta1.ConditionalContainer{
		{Container: corev1.Container{Name: "a"}},
		{Container: corev1.Container{Name: "b"}},
	}

	out := ApplyOptionsToAllConditional(in, WithNamePrefix("pre-"))

	if len(out) != len(in) {
		t.Fatalf("got %d containers, want %d", len(out), len(in))
	}
	want := []string{"pre-a", "pre-b"}
	got := []string{out[0].Name, out[1].Name}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}

func TestApplyOptionsToAllConditionalCopiesUnderlyingContainerFields(t *testing.T) {
	in := []v1beta1.ConditionalContainer{
		{Container: corev1.Container{Name: "a", Image: "example/a:latest"}},
	}

	out := ApplyOptionsToAllConditional(in, WithNamePrefix("pre-"))

	if len(out) != 1 {
		t.Fatalf("got %d containers, want 1", len(out))
	}
	if out[0].Image != "example/a:latest" {
		t.Errorf("Image = %q, want %q", out[0].Image, "example/a:latest")
	}
}

func TestApplyOptionsToAllConditionalDoesNotMutateInput(t *testing.T) {
	in := []v1beta1.ConditionalContainer{
		{Container: corev1.Container{Name: "a"}},
	}

	ApplyOptionsToAllConditional(in, WithNamePrefix("pre-"))

	if in[0].Name != "a" {
		t.Errorf("input was mutated: Name = %q, want %q", in[0].Name, "a")
	}
}

func TestApplyOptionsToAllConditionalAppliesAllOptions(t *testing.T) {
	in := []v1beta1.ConditionalContainer{
		{Container: corev1.Container{Name: "a"}},
	}

	out := ApplyOptionsToAllConditional(in,
		WithNamePrefix("first-"),
		WithWorkingDir("/workspace"),
	)

	if len(out) != 1 {
		t.Fatalf("got %d containers, want 1", len(out))
	}
	if out[0].Name != "first-a" {
		t.Errorf("Name = %q, want %q (earlier options are being discarded)", out[0].Name, "first-a")
	}
	if out[0].WorkingDir != "/workspace" {
		t.Errorf("WorkingDir = %q, want %q", out[0].WorkingDir, "/workspace")
	}
}

// KNOWN FAILURE: with zero options the inner loop never runs, so the source
// container is never copied into the result and callers get zero values back.
func TestApplyOptionsToAllConditionalWithNoOptions(t *testing.T) {
	in := []v1beta1.ConditionalContainer{
		{Container: corev1.Container{Name: "a", Image: "example/a:latest"}},
	}

	out := ApplyOptionsToAllConditional(in)

	if len(out) != 1 {
		t.Fatalf("got %d containers, want 1", len(out))
	}
	if out[0].Name != "a" || out[0].Image != "example/a:latest" {
		t.Errorf("container = %+v, want the source container to be copied through", out[0])
	}
}

func TestApplyOptionsToAllConditionalNilInput(t *testing.T) {
	if out := ApplyOptionsToAllConditional(nil, WithNamePrefix("pre-")); len(out) != 0 {
		t.Errorf("got %d containers, want 0", len(out))
	}
}

func TestWithPodSecurityStandardRestrictedCreatesSecurityContext(t *testing.T) {
	c := corev1.Container{}

	WithPodSecurityStandardRestricted()(&c)

	sc := c.SecurityContext
	if sc == nil {
		t.Fatal("SecurityContext is nil, want it to be populated")
	}
	if sc.AllowPrivilegeEscalation == nil {
		t.Fatal("AllowPrivilegeEscalation is nil, want a pointer to false")
	}
	if *sc.AllowPrivilegeEscalation {
		t.Error("AllowPrivilegeEscalation = true, want false")
	}
	if sc.Capabilities == nil {
		t.Fatal("Capabilities is nil, want ALL dropped")
	}
	if want := []corev1.Capability{"ALL"}; !reflect.DeepEqual(sc.Capabilities.Drop, want) {
		t.Errorf("Capabilities.Drop = %v, want %v", sc.Capabilities.Drop, want)
	}
	if sc.SeccompProfile == nil {
		t.Fatal("SeccompProfile is nil, want RuntimeDefault")
	}
	if sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("SeccompProfile.Type = %q, want %q",
			sc.SeccompProfile.Type, corev1.SeccompProfileTypeRuntimeDefault)
	}
}

func TestWithPodSecurityStandardRestrictedOverwritesUnsafeSettings(t *testing.T) {
	allow := true
	c := corev1.Container{
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: &allow,
			Capabilities:             &corev1.Capabilities{Add: []corev1.Capability{"NET_ADMIN"}},
			SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined},
		},
	}

	WithPodSecurityStandardRestricted()(&c)

	if *c.SecurityContext.AllowPrivilegeEscalation {
		t.Error("AllowPrivilegeEscalation = true, want it to be forced to false")
	}
	if len(c.SecurityContext.Capabilities.Add) != 0 {
		t.Errorf("Capabilities.Add = %v, want the added capabilities to be cleared",
			c.SecurityContext.Capabilities.Add)
	}
	if c.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("SeccompProfile.Type = %q, want %q",
			c.SecurityContext.SeccompProfile.Type, corev1.SeccompProfileTypeRuntimeDefault)
	}
}

func TestWithPodSecurityStandardRestrictedPreservesUnrelatedFields(t *testing.T) {
	uid := int64(1000)
	runAsNonRoot := true
	c := corev1.Container{
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:    &uid,
			RunAsNonRoot: &runAsNonRoot,
		},
	}

	WithPodSecurityStandardRestricted()(&c)

	if c.SecurityContext.RunAsUser == nil || *c.SecurityContext.RunAsUser != uid {
		t.Errorf("RunAsUser = %v, want %d", c.SecurityContext.RunAsUser, uid)
	}
	if c.SecurityContext.RunAsNonRoot == nil || !*c.SecurityContext.RunAsNonRoot {
		t.Errorf("RunAsNonRoot = %v, want true", c.SecurityContext.RunAsNonRoot)
	}
}

func TestWithPodSecurityStandardRestrictedIsIdempotent(t *testing.T) {
	first := ApplyOptionsTo(corev1.Container{}, WithPodSecurityStandardRestricted())
	second := ApplyOptionsTo(first, WithPodSecurityStandardRestricted())

	if !reflect.DeepEqual(first.SecurityContext, second.SecurityContext) {
		t.Errorf("SecurityContext changed on second application:\n first  = %+v\n second = %+v",
			first.SecurityContext, second.SecurityContext)
	}
}

func TestWithPodSecurityStandardRestrictedDoesNotShareStateBetweenContainers(t *testing.T) {
	opt := WithPodSecurityStandardRestricted()

	a := corev1.Container{Name: "a"}
	b := corev1.Container{Name: "b"}
	opt(&a)
	opt(&b)

	if a.SecurityContext == b.SecurityContext {
		t.Error("both containers share the same *SecurityContext pointer")
	}
	if a.SecurityContext.Capabilities == b.SecurityContext.Capabilities {
		t.Error("both containers share the same *Capabilities pointer")
	}
	if a.SecurityContext.AllowPrivilegeEscalation == b.SecurityContext.AllowPrivilegeEscalation {
		t.Error("both containers share the same AllowPrivilegeEscalation pointer")
	}
}

// ---------------------------------------------------------------------------
// WithContainerNameEnvVar
// ---------------------------------------------------------------------------

func TestWithContainerNameEnvVar(t *testing.T) {
	c := corev1.Container{Name: "scanner"}

	WithContainerNameEnvVar()(&c)

	got := lookupEnv(t, c.Env, v1beta1.EnvVarContainerName)
	if got.Value != "scanner" {
		t.Errorf("%s = %q, want %q", v1beta1.EnvVarContainerName, got.Value, "scanner")
	}
}

func TestWithContainerNameEnvVarPreservesExistingEnv(t *testing.T) {
	c := corev1.Container{
		Name: "scanner",
		Env:  []corev1.EnvVar{{Name: "EXISTING", Value: "value"}},
	}

	WithContainerNameEnvVar()(&c)

	if len(c.Env) != 2 {
		t.Fatalf("got %d env vars, want 2 (%v)", len(c.Env), envNames(c.Env))
	}
	if got := lookupEnv(t, c.Env, "EXISTING"); got.Value != "value" {
		t.Errorf("EXISTING = %q, want %q", got.Value, "value")
	}
}

// The env var captures the name at the moment the option runs, so ordering
// relative to WithNamePrefix is observable.
func TestWithContainerNameEnvVarOrderingWithNamePrefix(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		want    string
	}{
		{
			name:    "prefix applied first",
			options: []Option{WithNamePrefix("pre-"), WithContainerNameEnvVar()},
			want:    "pre-scanner",
		},
		{
			name:    "env var applied first",
			options: []Option{WithContainerNameEnvVar(), WithNamePrefix("pre-")},
			want:    "scanner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ApplyOptionsTo(corev1.Container{Name: "scanner"}, tt.options...)

			if got := lookupEnv(t, c.Env, v1beta1.EnvVarContainerName); got.Value != tt.want {
				t.Errorf("%s = %q, want %q", v1beta1.EnvVarContainerName, got.Value, tt.want)
			}
		})
	}
}

func TestApplyStandardOptions(t *testing.T) {
	out := ApplyStandardOptions([]corev1.Container{{Name: "a"}, {Name: "b"}})

	if len(out) != 2 {
		t.Fatalf("got %d containers, want 2", len(out))
	}

	for _, c := range out {
		sc := c.SecurityContext
		if sc == nil {
			t.Fatalf("container %q: SecurityContext is nil", c.Name)
		}
		if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
			t.Errorf("container %q: AllowPrivilegeEscalation = %v, want false",
				c.Name, sc.AllowPrivilegeEscalation)
		}
		if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
			t.Errorf("container %q: SeccompProfile = %+v, want RuntimeDefault", c.Name, sc.SeccompProfile)
		}
		if got := lookupEnv(t, c.Env, v1beta1.EnvVarContainerName); got.Value != c.Name {
			t.Errorf("container %q: %s = %q, want %q",
				c.Name, v1beta1.EnvVarContainerName, got.Value, c.Name)
		}
	}
}

func TestApplyStandardOptionsEmptySlice(t *testing.T) {
	if out := ApplyStandardOptions(nil); len(out) != 0 {
		t.Errorf("got %d containers, want 0", len(out))
	}
}

func TestWithParametersNoDefinitionsLeavesEnvUnchanged(t *testing.T) {
	c := corev1.Container{Env: []corev1.EnvVar{{Name: "EXISTING", Value: "value"}}}

	WithParameters(nil, nil, nil)(&c)

	if want := []string{"EXISTING"}; !reflect.DeepEqual(envNames(c.Env), want) {
		t.Errorf("env names = %v, want %v", envNames(c.Env), want)
	}
}

func TestWithParametersAppendsAfterExistingEnv(t *testing.T) {
	definitions := []v1beta1.ParameterDefinition{}
	settings := []v1beta1.ParameterSetting{}

	c := corev1.Container{Env: []corev1.EnvVar{{Name: "FIRST", Value: "1"}}}
	WithParameters(definitions, settings, map[string]string{})(&c)

	if len(c.Env) == 0 || c.Env[0].Name != "FIRST" {
		t.Errorf("existing env vars were not preserved in order: %v", envNames(c.Env))
	}
}
