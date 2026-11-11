package javaexec

import (
	"reflect"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	got := BuildArgs("/jdk/bin/java", `-Xmx1g -Dfoo="bar baz"`,
		[]string{"-jar", "app.jar"})
	want := []string{"/jdk/bin/java", "-Xmx1g", "-Dfoo=bar baz", "-jar", "app.jar"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildArgs = %#v, want %#v", got, want)
	}
}

func TestBuildArgsEmptyOptsHasNoEmptyArg(t *testing.T) {
	got := BuildArgs("/jdk/bin/java", "", []string{"-jar", "app.jar"})
	want := []string{"/jdk/bin/java", "-jar", "app.jar"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildArgs with empty JAVA_OPTS = %#v, want %#v", got, want)
	}
}

func TestBuildScriptArgs(t *testing.T) {
	got := BuildScriptArgs("/app/bin/play-app", `-Xmx1g -DcronExpr="0 */7 * * * *" -Dinject=$(hostname)`,
		[]string{"-Dother=1"})
	want := []string{"/app/bin/play-app", "-J-Xmx1g", "-J-DcronExpr=0 */7 * * * *", "-J-Dinject=$(hostname)", "-Dother=1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildScriptArgs = %#v, want %#v", got, want)
	}
}

func TestBuildScriptArgsEmptyOptsHasNoJFlag(t *testing.T) {
	got := BuildScriptArgs("/app/bin/play-app", "  ", nil)
	want := []string{"/app/bin/play-app"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildScriptArgs with empty JAVA_OPTS = %#v, want %#v", got, want)
	}
}
