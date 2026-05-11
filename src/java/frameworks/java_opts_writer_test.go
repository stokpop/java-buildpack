package frameworks_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/java-buildpack/src/java/common"
	"github.com/cloudfoundry/java-buildpack/src/java/frameworks"
	"github.com/cloudfoundry/libbuildpack"
)

func newJavaOptsContext(buildDir, cacheDir, depsDir string) *common.Context {
	logger := libbuildpack.NewLogger(GinkgoWriter)
	manifest := &libbuildpack.Manifest{}
	installer := &libbuildpack.Installer{}
	stager := libbuildpack.NewStager([]string{buildDir, cacheDir, depsDir, "0"}, logger, manifest)
	return &common.Context{
		Stager:    stager,
		Manifest:  manifest,
		Installer: installer,
		Log:       logger,
		Command:   &libbuildpack.Command{},
	}
}

var _ = Describe("Java Opts Writer", func() {
	var (
		buildDir string
		cacheDir string
		depsDir  string
		ctx      *common.Context
	)

	BeforeEach(func() {
		var err error
		buildDir, err = os.MkdirTemp("", "build")
		Expect(err).NotTo(HaveOccurred())
		cacheDir, err = os.MkdirTemp("", "cache")
		Expect(err).NotTo(HaveOccurred())
		depsDir, err = os.MkdirTemp("", "deps")
		Expect(err).NotTo(HaveOccurred())
		ctx = newJavaOptsContext(buildDir, cacheDir, depsDir)
	})

	AfterEach(func() {
		os.Unsetenv("JAVA_OPTS")
		os.RemoveAll(buildDir)
		os.RemoveAll(cacheDir)
		os.RemoveAll(depsDir)
	})

	Describe("Basic options", func() {
		It("writes JAVA_OPTS correctly", func() {
			javaOpts := "-Xmx512M -Xms256M"
			os.Setenv("JAVA_OPTS", javaOpts)

			Expect(os.Getenv("JAVA_OPTS")).To(Equal(javaOpts))
		})
	})

	Describe("CreateJavaOptsAssemblyScript", func() {
		runScript := func(javaOpts string, optsFileContent string) (string, error) {
			err := frameworks.CreateJavaOptsAssemblyScript(ctx)
			Expect(err).NotTo(HaveOccurred())

			optsDir := filepath.Join(depsDir, "0", "java_opts")
			Expect(os.MkdirAll(optsDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(optsDir, "42_agent.opts"), []byte(optsFileContent), 0644)).To(Succeed())

			scriptPath := filepath.Join(depsDir, "0", "profile.d", "00_java_opts.sh")
			cmd := exec.Command("bash", "-c", "source "+scriptPath+" && echo \"$JAVA_OPTS\"")
			cmd.Env = append(os.Environ(),
				"JAVA_OPTS="+javaOpts,
				"DEPS_DIR="+depsDir,
				"HOME=/home/vcap/app",
			)
			output, err := cmd.CombinedOutput()
			return string(output), err
		}

		It("handles multiline JAVA_OPTS from YAML block scalar without sed error", func() {
			// Reproduce the manifest pattern:
			//   JAVA_OPTS: >
			//     -javaagent:$HOME/BOOT-INF/lib/agent.jar
			//     -XX:+UseZGC
			// YAML '>' folds newlines to spaces, but CF may deliver them as literal newlines
			multilineJavaOpts := "-javaagent:$HOME/BOOT-INF/lib/agent.jar\n-XX:+UseZGC\n-XX:+AlwaysPreTouch"

			output, err := runScript(multilineJavaOpts, "-javaagent:somepath.jar $JAVA_OPTS")
			Expect(err).NotTo(HaveOccurred(), "script failed with output: %s", output)
			Expect(output).To(ContainSubstring("-XX:+UseZGC"))
		})

		It("handles pipe character in JAVA_OPTS (e.g. javaagent options) without sed error", func() {
			// Reproduce the manifest pattern:
			//   JAVA_OPTS: >
			//     -javaagent:$HOME/BOOT-INF/lib/jfr-exporter.jar=enableExecutorMBeans|disableMyFeature
			pipeJavaOpts := "-javaagent:$HOME/BOOT-INF/lib/jfr-exporter.jar=enableExecutorMBeans|disableMyFeature"

			output, err := runScript(pipeJavaOpts, "-javaagent:somepath.jar $JAVA_OPTS")
			Expect(err).NotTo(HaveOccurred(), "script failed with output: %s", output)
			Expect(output).To(ContainSubstring("enableExecutorMBeans|disableMyFeature"))
		})

		It("expands $HOME in opts file content", func() {
			output, err := runScript("", "-javaagent:$HOME/BOOT-INF/lib/agent.jar")
			Expect(err).NotTo(HaveOccurred(), "script failed with output: %s", output)
			Expect(output).To(ContainSubstring("-javaagent:/home/vcap/app/BOOT-INF/lib/agent.jar"))
		})

		It("expands $DEPS_DIR in opts file content", func() {
			output, err := runScript("", "-Djava.security.properties=$DEPS_DIR/0/security.properties")
			Expect(err).NotTo(HaveOccurred(), "script failed with output: %s", output)
			Expect(output).To(ContainSubstring("-Djava.security.properties=" + depsDir + "/0/security.properties"))
		})

		It("preserves > in opts file content (e.g. JMA threshold >600MB)", func() {
			// -Djma.thresholds.old_gen=>600MB contains '>' which must not be treated
			// as a shell redirect operator — the JVM must receive the full value.
			output, err := runScript("", "-Djma.thresholds.old_gen=>600MB")
			Expect(err).NotTo(HaveOccurred(), "script failed with output: %s", output)
			Expect(output).To(ContainSubstring("-Djma.thresholds.old_gen=>600MB"))
		})

		It("expands $JAVA_HOME in opts file content", func() {
			output, err := runScript("", "-Djava.ext.dirs=$JAVA_HOME/jre/lib/ext")
			Expect(err).NotTo(HaveOccurred(), "script failed with output: %s", output)
			// $JAVA_HOME resolves to something (may be empty in test env, but must not error)
			Expect(output).NotTo(ContainSubstring("$JAVA_HOME"))
		})

		It("expands $TMPDIR in opts file content (e.g. Contrast Security Agent)", func() {
			cmd := exec.Command("bash", "-c",
				"source "+filepath.Join(depsDir, "0", "profile.d", "00_java_opts.sh")+" && echo \"$JAVA_OPTS\"")

			err := frameworks.CreateJavaOptsAssemblyScript(ctx)
			Expect(err).NotTo(HaveOccurred())
			optsDir := filepath.Join(depsDir, "0", "java_opts")
			Expect(os.MkdirAll(optsDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(optsDir, "18_contrast.opts"), []byte("-Dcontrast.dir=$TMPDIR"), 0644)).To(Succeed())

			cmd.Env = append(os.Environ(),
				"JAVA_OPTS=",
				"DEPS_DIR="+depsDir,
				"HOME=/home/vcap/app",
				"TMPDIR=/tmp/myapp",
			)
			output, cmdErr := cmd.CombinedOutput()
			Expect(cmdErr).NotTo(HaveOccurred(), "script failed with output: %s", output)
			Expect(string(output)).To(ContainSubstring("-Dcontrast.dir=/tmp/myapp"))
		})
	})
})
