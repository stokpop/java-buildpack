package javaexec

// BuildArgs assembles the full argv for execing the JVM. The resulting slice is
// [java, <tokenized JAVA_OPTS...>, <trusted args...>], mirroring the order of
// the previous `exec java $JAVA_OPTS <trusted>` start command. trusted args are
// produced by the buildpack (classpath, -jar/-cp, main class) and are passed
// through unchanged; only JAVA_OPTS is tokenized.
func BuildArgs(java, javaOpts string, trusted []string) []string {
	opts := TokenizeJavaOpts(javaOpts)
	argv := make([]string, 0, 1+len(opts)+len(trusted))
	argv = append(argv, java)
	argv = append(argv, opts...)
	argv = append(argv, trusted...)
	return argv
}

// BuildScriptArgs assembles the argv for execing a bundled start script (e.g. an
// sbt-native-packager script) instead of the JVM. Each tokenized JAVA_OPTS entry
// is passed as a -J<flag> argument, which those scripts collect into a properly
// quoted array, rather than relying on the script word-splitting $JAVA_OPTS.
func BuildScriptArgs(script, javaOpts string, trusted []string) []string {
	opts := TokenizeJavaOpts(javaOpts)
	argv := make([]string, 0, 1+len(opts)+len(trusted))
	argv = append(argv, script)
	for _, opt := range opts {
		argv = append(argv, "-J"+opt)
	}
	argv = append(argv, trusted...)
	return argv
}
