package deploytools

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/term"
)

var bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func ValidBucket(s string) bool {
	if !bucketPattern.MatchString(s) || strings.Contains(s, "..") || strings.Contains(s, ".-") || strings.Contains(s, "-.") || net.ParseIP(s) != nil {
		return false
	}
	for _, p := range []string{"xn--", "sthree-", "amzn-s3-demo-"} {
		if strings.HasPrefix(s, p) {
			return false
		}
	}
	for _, p := range []string{"-s3alias", "--ol-s3", ".mrap", "--x-s3", "--table-s3"} {
		if strings.HasSuffix(s, p) {
			return false
		}
	}
	return true
}
func CheckTools() error {
	missing := []string{}
	for _, name := range []string{"go", "aws", "pulumi"} {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("instala primero: %s. Consulta README.md", strings.Join(missing, ", "))
	}
	return nil
}

type commandResult struct {
	stdout, stderr string
	err            error
}
type wizard struct {
	root   string
	input  io.Reader
	output io.Writer
}
type aborted struct{ err error }

func stop(err error) {
	if err != nil {
		panic(aborted{err})
	}
}
func (w *wizard) step(n int, title string) {
	if w.output == os.Stdout && term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == "" {
		fmt.Fprintf(w.output, "\n\033[1;32m%d  %s\033[0m\n%s\n", n, title, strings.Repeat("─", 55))
	} else {
		fmt.Fprintf(w.output, "\n%d  %s\n%s\n", n, title, strings.Repeat("─", 55))
	}
}
func (w *wizard) ask(prompt, def string) string {
	fmt.Fprint(w.output, prompt)
	if def != "" {
		fmt.Fprintf(w.output, " [%s]", def)
	}
	fmt.Fprint(w.output, ": ")
	// Read exactly one line; do not prefetch data intended for a hidden prompt.
	var b strings.Builder
	one := make([]byte, 1)
	for {
		n, err := w.input.Read(one)
		if n > 0 {
			if one[0] == '\n' {
				break
			}
			b.WriteByte(one[0])
		}
		if err != nil {
			stop(fmt.Errorf("entrada cancelada: %w", err))
		}
	}
	answer := strings.TrimSpace(b.String())
	if answer == "" {
		return def
	}
	return answer
}
func (w *wizard) secret(env, label string) string {
	if s := os.Getenv(env); s != "" {
		return s
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		stop(fmt.Errorf("%s requiere una terminal para ocultar la entrada; también puedes configurar %s", label, env))
	}
	fmt.Fprintf(w.output, "%s (oculto): ", label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(w.output)
	stop(err)
	if len(b) == 0 {
		stop(errors.New("el secreto no puede estar vacío"))
	}
	return string(b)
}
func (w *wizard) command(dir, input string, capture bool, args ...string) commandResult {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	} else {
		cmd.Stdin = os.Stdin
	}
	var out, stderr bytes.Buffer
	if capture {
		cmd.Stdout = &out
		cmd.Stderr = &stderr
	} else {
		cmd.Stdout = w.output
		cmd.Stderr = w.output
	}
	err := cmd.Run()
	return commandResult{out.String(), stderr.String(), err}
}
func (w *wizard) must(input string, capture bool, args ...string) string {
	r := w.command(filepath.Join(w.root, "infra"), input, capture, args...)
	if r.err != nil {
		stop(fmt.Errorf("falló %s: %w. Revisa credenciales, permisos y el estado; no se revirtieron recursos", args[0], r.err))
	}
	return r.stdout
}

func Deploy(root string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if a, ok := r.(aborted); ok {
				err = a.err
			} else {
				panic(r)
			}
		}
	}()
	w := &wizard{root: root, input: os.Stdin, output: os.Stdout}
	fmt.Fprintln(w.output, "\n✦ ABI · Despliegue de la aldea\n  Go + AWS + Cloudflare · estado privado en S3")
	w.step(1, "Revisar herramientas")
	stop(CheckTools())
	w.step(2, "Conectar con AWS")
	profile := w.ask("Perfil AWS SSO", "default")
	stop(os.Setenv("AWS_PROFILE", profile))
	stop(os.Setenv("AWS_PAGER", ""))
	if r := w.command(root, "", true, "aws", "sts", "get-caller-identity"); r.err != nil {
		fmt.Fprintln(w.output, "Si no configuraste el perfil, ejecuta aws configure sso primero.")
		w.must("", false, "aws", "sso", "login", "--profile", profile)
	}
	var identity struct{ Account string }
	stop(json.Unmarshal([]byte(w.must("", true, "aws", "sts", "get-caller-identity")), &identity))
	if identity.Account == "" {
		stop(errors.New("AWS no devolvió el ID de cuenta"))
	}
	fmt.Fprintln(w.output, "Cuenta AWS:", identity.Account)
	region := w.ask("Región AWS", "us-east-1")
	stop(os.Setenv("AWS_REGION", region))
	stop(os.Setenv("AWS_DEFAULT_REGION", region))
	w.step(3, "Preparar estado Pulumi privado")
	bucket := w.ask("Bucket de estado", "abi-pulumi-"+identity.Account+"-"+region)
	if !ValidBucket(bucket) {
		stop(errors.New("nombre de bucket inválido"))
	}
	head := w.command(root, "", true, "aws", "s3api", "head-bucket", "--bucket", bucket)
	if head.err != nil {
		if !strings.Contains(head.stderr, "404") && !strings.Contains(head.stderr, "Not Found") {
			stop(errors.New("no se pudo acceder al bucket; revisa permisos. No se intentará crearlo a ciegas"))
		}
		if w.ask("Crear el bucket de estado (genera recursos AWS)? Escribe CREAR", "") != "CREAR" {
			return nil
		}
		args := []string{"aws", "s3api", "create-bucket", "--bucket", bucket}
		if region != "us-east-1" {
			args = append(args, "--create-bucket-configuration", "LocationConstraint="+region)
		}
		w.must("", true, args...)
	}
	w.must("", true, "aws", "s3api", "put-public-access-block", "--bucket", bucket, "--public-access-block-configuration", "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true")
	w.must("", true, "aws", "s3api", "put-bucket-versioning", "--bucket", bucket, "--versioning-configuration", "Status=Enabled")
	w.must("", true, "aws", "s3api", "put-bucket-encryption", "--bucket", bucket, "--server-side-encryption-configuration", `{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}`)
	stop(os.Setenv("PULUMI_CONFIG_PASSPHRASE", w.secret("PULUMI_CONFIG_PASSPHRASE", "Frase de cifrado Pulumi (guárdala y reutilízala)")))
	w.must("", false, "pulumi", "login", "s3://"+bucket+"/birthday-invitation")
	stack := w.ask("Stack", "production")
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(stack) {
		stop(errors.New("nombre de stack inválido"))
	}
	// A failed select can mean a network/auth error, not a missing stack.
	var stacks []struct {
		Name string `json:"name"`
	}
	stop(json.Unmarshal([]byte(w.must("", true, "pulumi", "stack", "ls", "--json")), &stacks))
	exists := false
	for _, s := range stacks {
		if s.Name == stack || strings.HasSuffix(s.Name, "/"+stack) {
			exists = true
		}
	}
	if exists {
		w.must("", false, "pulumi", "stack", "select", stack)
	} else {
		w.must("", false, "pulumi", "stack", "init", stack, "--secrets-provider", "passphrase")
	}
	w.step(4, "Dominio y secretos")
	domain := w.ask("Dominio", "xitlaliabigail.com")
	zone := w.ask("Cloudflare Zone ID", "")
	if zone == "" {
		stop(errors.New("falta Cloudflare Zone ID"))
	}
	stop(os.Setenv("CLOUDFLARE_API_TOKEN", w.secret("CLOUDFLARE_API_TOKEN", "Token Cloudflare (DNS Edit y Zone Read, solo esta zona)")))
	for _, pair := range [][2]string{{"aws:region", region}, {"domain", domain}, {"cloudflareZoneId", zone}} {
		w.must("", false, "pulumi", "config", "set", pair[0], pair[1])
	}
	var configs map[string]json.RawMessage
	stop(json.Unmarshal([]byte(w.must("", true, "pulumi", "config", "--json")), &configs))
	for _, name := range []string{"adminKey", "importNamespace"} {
		if _, ok := configs["birthday-invitation:"+name]; ok {
			continue
		}
		if _, ok := configs[name]; ok {
			continue
		}
		b := make([]byte, 40)
		_, e := rand.Read(b)
		stop(e)
		w.must(base64.RawURLEncoding.EncodeToString(b)+"\n", true, "pulumi", "config", "set", name, "--secret")
	}
	fmt.Fprintln(w.output, "Claves cifradas. No se muestran en registros.")
	w.step(5, "Compilar y revisar")
	stop(Build(root))
	r := w.command(root, "", false, "go", "build", "-o", filepath.Join(root, "build", "infra-check"), "./infra")
	stop(r.err)
	w.must("", false, "pulumi", "preview", "--diff")
	w.step(6, "Desplegar")
	if w.ask("Revisa los cambios. Escribe DESPLEGAR para aplicarlos", "") != "DESPLEGAR" {
		fmt.Fprintln(w.output, "Sin cambios en la aplicación. El bucket de estado puede haber sido preparado.")
		return nil
	}
	w.must("", false, "pulumi", "up", "--yes", "--skip-preview")
	fmt.Fprintln(w.output, "\n✓ Listo. Abre https://"+domain+"/admin.html")
	fmt.Fprintln(w.output, "Desde infra: pulumi config get adminKey (en privado). Para importar: pulumi config get importNamespace y pulumi stack output tableName.")
	return nil
}
