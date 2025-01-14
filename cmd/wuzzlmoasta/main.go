package main

//go:generate go run ../../tools/generate-version.go

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"git.sr.ht/~patrickpichler/wuzzlmoasta/pkg/ui"
	"git.sr.ht/~patrickpichler/wuzzlmoasta/pkg/users"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/template/html"
)

const (
	UserSessionIdCookie = "UserSessionId"
	UserLoggedIn        = "loggedIn"
	User                = "user"
)

func main() {
	flags := flag.NewFlagSet("", flag.PanicOnError)

	resourcesDir := flags.String("resourcesDir", "", "path to resources dir")

	flags.Parse(os.Args[1:])

	var engine *html.Engine
	var staticFilesFs http.FileSystem

	if *resourcesDir != "" {
		if _, err := os.Stat(*resourcesDir); errors.Is(err, os.ErrNotExist) {
			panic(fmt.Sprintf("folder `%s` does not exist", *resourcesDir))
		}

		engine = html.New(filepath.Join(*resourcesDir, "views"), ".html")
		engine.Reload(true)

		staticFilesFs = http.Dir(filepath.Join(*resourcesDir, "static"))
	} else {
		engine = ui.CreateEmbeddedEngine()
		staticFilesFs = ui.GetStaticFilesFs()
	}

	app := fiber.New(fiber.Config{
		Views:        engine,
		ErrorHandler: errorHandler,
	})

	app.Use("/static", filesystem.New(filesystem.Config{
		Root: staticFilesFs,
	}))

	app.Get("/", checkLoginOrRedirectToLoginPage, func(c *fiber.Ctx) error {
		user := c.Locals(User).(users.ViewableUser)

		return c.Render("index", fiber.Map{
			"user": user,
		}, "layouts/main")
	})

	app.Get("/login", func(c *fiber.Ctx) error {
		return c.Render("login", fiber.Map{}, "layouts/main")
	})

	app.Post("/login", func(c *fiber.Ctx) error {
		username := c.FormValue("username")
		password := c.FormValue("password")

		cookie, err := users.TryLogin(username, password)

		if errors.Is(err, users.InvalidUsernameOrPassword) {
			return c.Render("login", fiber.Map{
				"invalidLogin": true,
			}, "layouts/main")
		}

		c.Cookie(&fiber.Cookie{
			Name:  UserSessionIdCookie,
			Value: cookie,
		})

		return c.Redirect("/")
	})

	app.Use(func(c *fiber.Ctx) error {
		return c.Status(404).Render("errors/404", fiber.Map{})
	})

	log.Fatal(app.Listen(":8080"))
}

func checkLoginOrRedirectToLoginPage(c *fiber.Ctx) error {
	sessionId := c.Cookies(UserSessionIdCookie)

	if valid, user := users.IsTokenValid(sessionId); valid {
		c.Locals(User, *user)
		return c.Next()
	}

	return c.Redirect("/login")
}

func setLoggedIn(c *fiber.Ctx) error {
	sessionId := c.Cookies(UserSessionIdCookie)

	if valid, user := users.IsTokenValid(sessionId); valid {
		c.Locals(UserLoggedIn, "1")
		c.Locals(User, user)
	} else {
		c.Locals(UserLoggedIn, "")
	}

	return c.Next()
}

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	if code == fiber.StatusInternalServerError {
		log.Println(err)
	}

	err = c.Status(code).Render(fmt.Sprintf("errors/%d", code), fiber.Map{})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
	}

	return nil
}
