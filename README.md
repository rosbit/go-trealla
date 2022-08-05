# An embeddable Prolog

[trealla](https://github.com/trealla-prolog/trealla) is A compact, efficient Prolog interpreter written in plain-old C.

This package is intended to provide a wrapper to interact `trealla` with application written in golang.
With some helper functions, `go-trealla` makes it simple to calle Prolog from Golang, and `go-trallea` can be
treated as an embeddable Prolog.

### Usage

The package is fully go-getable, so, just type

  `go get github.com/rosbit/go-trealla`

to install.

#### 1. Instantiate a Prolog interpreter

```go
package main

import (
  "github.com/rosbit/go-trealla"
  "fmt"
)

func main() {
  ctx, err := trealla.NewTrealla("/path/to/trealla/exe")
  if err != nil {
      // error processing
  }
  ...
}
```

#### 2. Load a Prolog script

Suppose there's a Prolog file named `music.pl` like this:

```prolog
listen(ergou, bach).
listen(ergou, beethoven).
listen(ergou, mozart).
listen(xiaohong, mj).
listen(xiaohong, dylan).
listen(xiaohong, bach).
listen(xiaohong, beethoven).
```

one can load the script like this:

```go
   if err := ctx.LoadFile("music.pl"); err != nil {
      // error processing
   }
```

#### 3. Prepare arguments and variables

```go
   // query Who listens to Music
   args := []interface{}{trealla.PlVar("Who"), trealla.PlVar("Music")}

   // query Who listens to "bach"
   args := []interface{}{trealla.PlVar("Who"), "bach"}

   // query Which Music "ergou" listens to
   args := []interface{}{"ergou", trealla.PlVar("Music")}

   // check whether "ergou" listens to "bach"
   args := []interface{}{"ergou", "bach"}
```

#### 4. Query the goal with arguments and variables

```go
   rs, ok, err := ctx.Query("listen", args...)
```

#### 5. Check the result

```go
   // error checking
   if err != nil {
      // error processing
      return
   }

   // proving checking with result `false`
   if !ok {
      // the result is false
      return
   }

   // proving checking with result `true`
   if rs == nil {
      // the result is true
      return
   }

   // result set processing
   for res := range rs {
      fmt.Printf("res: %#v\n", res)
   }
```

The full usage sample can be found [sample/main.go](sample/main.go).

### Status

The package is not fully tested, so be careful.

### Contribution

Pull requests are welcome! Also, if you want to discuss something send a pull request with proposal and changes.

__Convention:__ fork the repository and make changes on your fork in a feature branch.
