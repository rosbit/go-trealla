package trealla

import (
	"github.com/rosbit/go-expect"
	"strconv"
	"fmt"
	"time"
	"bytes"
	"regexp"
	"strings"
	"runtime"
)

const (
	timeout = 1 * time.Second
)

var (
	promptRE   = regexp.MustCompile(`^\?\- `)                    // "?- "           prompt
	trueRE     = regexp.MustCompile(`^\s+true\.[\r\n]`)          // " true."        true result
	falseRE    = regexp.MustCompile(`^\s+false\.[\r\n]`)         // " false."       false result
	wantMoreRE = regexp.MustCompile(`^;`)                        // ";"             waiting ";" for more
	errRE      = regexp.MustCompile(`^\s+error\(.+?\)\.[\r\n]`)  // " error(....)"  error message
	resultRE   = regexp.MustCompile(`^\s+[A-Z_][^ ]* = [^ ]+?([^\r\n])*[\r\n]`)  // Var1 = val1[, Var2 = val2]...    result with vars bound
	consultRE  = regexp.MustCompile("^\\[[^\\]]+\\]\\.[\r\n]")      // ['filename.pl'].
	goalRE     = regexp.MustCompile(`^[a-z][^\(]*\(.*?\)\.[\r\n]`)  // goal(xxx,xxx).
	msgRE      = regexp.MustCompile(`^.*?[\r\n]`)

	consultCases = []*expect.Case{
		&expect.Case{Exp: promptRE, MatchedOnly: true},
		&expect.Case{Exp: trueRE},
		&expect.Case{Exp: errRE},
		&expect.Case{Exp: consultRE, SkipTill: '\n'},
		&expect.Case{Exp: goalRE, SkipTill: '\n'},
	}

	goalCases = []*expect.Case{
		&expect.Case{Exp: promptRE, MatchedOnly: true},
		&expect.Case{Exp: wantMoreRE, MatchedOnly: true},
		&expect.Case{Exp: falseRE},
		&expect.Case{Exp: trueRE},
		&expect.Case{Exp: errRE},
		&expect.Case{Exp: resultRE},
		&expect.Case{Exp: goalRE, SkipTill: '\n'},
		&expect.Case{Exp: consultRE, SkipTill: '\n'},
		&expect.Case{Exp: msgRE},
	}
)

type Trealla struct {
	e *expect.Expect
}

func NewTrealla(treallaExePath string) (*Trealla, error) {
	e, err := spawn(treallaExePath)
	if err != nil {
		return nil, err
	}
	e.SetTimeout(timeout)

	for i:=0; i<2; i++ {
		if _, err := e.ExpectRegexp(promptRE); err != nil {
			if err == expect.NotFound || err == expect.TimedOut {
				continue
			}
			e.Close()
			return nil, err
		}
		break
	}

	t := &Trealla{e: e}
	runtime.SetFinalizer(t, closeTrealla)
	return t, nil
}

func closeTrealla(t *Trealla) {
	t.e.Send("halt.\n")
	t.e.Close()
}

func (t *Trealla) LoadFile(plFile string) (err error) {
	t.e.Send(fmt.Sprintf("['%s'].\n", plFile))  // ['prolog-file-name'].
CHECK:
	idx, m, e := t.e.ExpectCases(consultCases...)
	if e != nil {
		// timeout
		if e == expect.TimedOut || e == expect.NotFound {
			goto CHECK
		}
		err = e
		return
	}

	switch idx {
	case 0:
		//?-
		return
	case 1:
		// true.
	case 2:
		// error occurred
		err = fmt.Errorf("%s", m)
		return
	case 3:
		// [consult].
		res, _ := extractConsultResult(m)
		if len(res) == 0 {
			goto CHECK
		}

		if strings.HasPrefix(res, " true.") {
			break
		}
	default:
		err = fmt.Errorf("idx: %d", idx)
		return
	}

	if !bytes.HasSuffix(m, []byte("?- ")) {
		_, err = t.e.ExpectRegexp(promptRE)
	}

	return
}

func extractConsultResult(m []byte) (string, string) {
	pos := bytes.IndexByte(m, '\n')
	if pos < 0 {
		return "", ""
	}
	m = m[pos+1:]
	if len(m) == 0 {
		return "", ""
	}
	pos = bytes.IndexByte(m, '\n')
	if pos >= 0 {
		pos += 1
		return string(m[:pos]), string(m[pos:])
	}
	return string(m), ""
}

func (t *Trealla) Query(predict string, args ...interface{}) (it <-chan map[string]interface{}, ok bool, err error) {
	if len(predict) == 0 {
		err = fmt.Errorf("predict name expected")
		return
	}
	defer func() {
		if r := recover(); r != nil {
			if v, o := r.(error); o {
				err = v
				return
			}
			err = fmt.Errorf("%v", r)
			return
		}
	}()

	goal, e := makeGoal(predict, args...)
	if e != nil {
		err = e
		return
	}
	it, ok, err = t.doQuery(goal)
	return
}

func makeGoal(predict string, args ...interface{}) (goal string, err error) {
	argc := len(args)
	if argc == 0 {
		goal = fmt.Sprintf("%s()", predict)
		return
	}

	argv := make([]Term, argc)
	for i, arg := range args {
		if plVar, ok := arg.(PlVar); ok {
		    vName := string(plVar)
		    if len(vName) == 0 {
				vName = fmt.Sprintf("_Var%d", i)
		    }
		    argv[i] = PlVar(vName).ToTerm()
		} else {
		    argv[i], err = makePlTerm(arg)
		    if err != nil {
				return
		    }
		}
	}
	goal = fmt.Sprintf("%s(%s)", predict, strings.Join(argv, ","))
	return
}

func (t *Trealla) doQuery(goal string) (it <-chan map[string]interface{}, ok bool, err error) {
	t.e.Send(fmt.Sprintf("%s.\n", goal))

	res := make(chan map[string]interface{})
	it = res

	statusDone := make(chan struct{})
	hasStatus := false

	go func() {
		for {
			idx, m, e := t.e.ExpectCases(goalCases...)
			if e != nil {
				if e == expect.TimedOut || e == expect.NotFound {
					continue
				}
				if !hasStatus {
					err = e
					hasStatus = true
					close(statusDone)
				}
				break
			}

			if idx == 0 {
				// prompt
				if !hasStatus {
					hasStatus = true
					close(statusDone)
				}
				break
			}

			switch idx {
			case 5:
				// result Var1 = val1[, Var2 = val2]...
				if !hasStatus {
					hasStatus = true
					ok = true
					close(statusDone)
				}
				if kv := extractVars(m); len(kv) > 0 {
					res <- kv
				}
				if bytes.HasSuffix(m, []byte(";")) {
					t.e.Send(";")
					continue
				}
			case 1:
				//;
				t.e.Send(";")
			case 2:
				// false.
				ok = false
				it = nil
			case 3:
				// true.
				ok = true
				it = nil
			case 4:
				// error(xxx).
				ok = false
				err = fmt.Errorf("%s", extractError(m))
				it = nil
			case 8:
				// msg
				// fmt.Printf(">>>msg: %s<<<\n", m)
				realMsg, maybeResult, prompt := extractMessage(m)
				fmt.Printf("%s", realMsg)
				if len(prompt) == 0 && len(maybeResult) == 0 {
					continue
				}
				// fmt.Printf(">>maybeResult: %s, prompt: %s<<<\n", maybeResult, prompt)
				switch {
				case strings.HasPrefix(maybeResult, " true."):
					ok = true
					it = nil
				case strings.HasPrefix(maybeResult, " false."):
					ok = false
					it = nil
				case prompt == ";":
					t.e.Send(";")
					fallthrough
				default:
					if len(maybeResult) > 0 {
						fmt.Printf("%s", maybeResult)
					}
					continue
				}
			default:
				// TimedOut or NotFound
				continue
			}

			if !hasStatus {
				hasStatus = true
				close(statusDone)
			}
			if bytes.HasSuffix(m, []byte("?- ")) {
				break
			}
		}

		if !hasStatus {
			close(statusDone)
		}
		close(res)
	}()

	<-statusDone
	return
}

func extractError(m []byte) string {
	pos := bytes.IndexByte(m, '\n')
	if pos >= 0 {
		return string(bytes.TrimSpace(m[:pos]))
	}
	return string(bytes.TrimSpace(m))
}

func extractMessage(m []byte) (string, string, string) {
	pos := bytes.LastIndexByte(m, '\n')
	if pos >= 0 {
		pos += 1
		msg, lastMsg := string(m[:pos]), string(m[pos:])
		if len(lastMsg) > 0 && !strings.HasPrefix(lastMsg, "?- ") && lastMsg != ";" {
			return msg, lastMsg, ""
		}
		pos = strings.LastIndexByte(msg[:len(msg)-1], '\n')
		if pos >= 0 {
			pos += 1
			return msg[:pos], msg[pos:], lastMsg
		}
		return msg, "", lastMsg
	}
	return string(m), "", ""
}

func extractVars(m []byte) map[string]interface{} {
	pos := bytes.LastIndexByte(m, '\n')
	if pos >= 0 {
		m = m[:pos]
	}
	m = bytes.TrimSpace(m)
	if bytes.HasSuffix(m, []byte(".")) {
		m = m[:len(m)-1]
	}

	pairs := bytes.Split(m, []byte(", "))
	if len(pairs) == 0 {
		return nil
	}

	res := map[string]interface{}{}
	for _, pair := range pairs {
		kv := bytes.Split(pair, []byte(" = "))
		if len(kv) == 2 {
			res[string(kv[0])] = parseValue(string(kv[1]))
		}
	}
	return res
}

func parseValue(v string) interface{} {
	if len(v) == 0 {
		return v
	}
	switch v[0] {
	case '"':
		if s, err := strconv.Unquote(v); err == nil {
			return s
		}
	case '\'':
		l := len(v)
		if l > 1 && v[l-1] == '\'' {
			return v[1:l-1]
		}
	case '-':
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	case '0','1','2','3','4','5','6','7','8','9':
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		if i, err := strconv.ParseUint(v, 10, 64); err == nil {
			return i
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	case 't':
		if v == "true" {
			return true
		}
	case 'f':
		if v == "false" {
			return false
		}
	default:
		return v
	}

	return v
}
