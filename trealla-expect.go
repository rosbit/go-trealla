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
	for {
		_, _, e := t.e.ExpectCases(
			&expect.Case{Exp: promptRE, MatchedOnly: true, ExpMatched: func(_ []byte) expect.Action{
				return expect.Break
			}},
			&expect.Case{Exp: trueRE, SkipTill: '\n'},
			&expect.Case{Exp: errRE, ExpMatched: func(m []byte) expect.Action{
				err = fmt.Errorf("%s", extractError(m))
				return expect.Continue
			}},
			&expect.Case{Exp: consultRE, SkipTill: '\n'},
		)
		if e != nil {
			// timeout
			if e == expect.TimedOut || e == expect.NotFound {
				continue
			}
			err = e
		}
		break
	}

	return
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
			_, _, e := t.e.ExpectCases(
				&expect.Case{Exp: promptRE, MatchedOnly: true, ExpMatched: func(_ []byte) expect.Action{
					if !hasStatus {
						hasStatus = true
						close(statusDone)
					}
					return expect.Break
				}},
				&expect.Case{Exp: wantMoreRE, MatchedOnly: true, ExpMatched: func(_ []byte) expect.Action{
					t.e.Send(";")
					return expect.Continue
				}},
				&expect.Case{Exp: falseRE, ExpMatched: func(_ []byte) expect.Action{
					ok = false
					it = nil
					return expect.Continue
				}},
				&expect.Case{Exp: trueRE, ExpMatched: func(_ []byte) expect.Action{
					ok = true
					it = nil
					return expect.Continue
				}},
				&expect.Case{Exp: errRE, ExpMatched: func(m []byte) expect.Action{
					ok = false
					err = fmt.Errorf("%s", extractError(m))
					it = nil
					return expect.Continue
				}},
				&expect.Case{Exp: resultRE, ExpMatched: func(m []byte) expect.Action{
					if !hasStatus {
						hasStatus = true
						ok = true
						close(statusDone)
					}
					if kv := extractVars(m); len(kv) > 0 {
						res <- kv
					}
					return expect.Continue
				}},
				&expect.Case{Exp: goalRE, SkipTill: '\n'},
				&expect.Case{Exp: consultRE, SkipTill: '\n'},
				&expect.Case{Exp: msgRE, ExpMatched: func(m []byte) expect.Action{
					fmt.Printf("%s", m)
					return expect.Continue
				}},
			)

			if e != nil {
				if e == expect.TimedOut || e == expect.NotFound {
					continue
				}
			}

			if !hasStatus {
				err = e
				hasStatus = true
				close(statusDone)
			}
			break
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
