package provider

import "testing"

var (
	benchDeny = []string{
		"git push --force", "git push -f", "git branch -D",
		"git reset --hard", "sudo:*", "rm -rf *",
		"gh pr merge", "gh pr close", "gh issue close",
	}
	benchAsk = []string{
		"git push", "gh pr *", "npm publish",
		"docker push *", "kubectl delete *",
	}
	benchCmds = []string{
		"git status",
		"git push --force origin main",
		"echo hello && git status",
		"npm run build",
		"docker compose up -d",
		"kubectl get pods",
	}
)

func BenchmarkPermission_NoMatch(b *testing.B) {
	m := buildMatcher(benchDeny, benchAsk)
	for b.Loop() {
		matchCmd("git status", m)
	}
}

func BenchmarkPermission_DenyMatch(b *testing.B) {
	m := buildMatcher(benchDeny, benchAsk)
	for b.Loop() {
		matchCmd("git push --force origin main", m)
	}
}

func BenchmarkPermission_AskWildcard(b *testing.B) {
	m := buildMatcher(benchDeny, benchAsk)
	for b.Loop() {
		matchCmd("gh pr create --title foo", m)
	}
}

func BenchmarkPermission_Compound(b *testing.B) {
	m := buildMatcher(benchDeny, benchAsk)
	for b.Loop() {
		matchCmd("echo hello && git status && npm test", m)
	}
}

func BenchmarkPermission_WildcardDeny(b *testing.B) {
	m := buildMatcher(benchDeny, benchAsk)
	for b.Loop() {
		matchCmd("rm -rf /tmp/foo", m)
	}
}

func BenchmarkPermission_Mixed(b *testing.B) {
	m := buildMatcher(benchDeny, benchAsk)
	for b.Loop() {
		for _, cmd := range benchCmds {
			matchCmd(cmd, m)
		}
	}
}

func BenchmarkPermission_BuildAndMatch(b *testing.B) {
	for b.Loop() {
		m := buildMatcher(benchDeny, benchAsk)
		for _, cmd := range benchCmds {
			matchCmd(cmd, m)
		}
	}
}
