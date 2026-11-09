package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/swaggest/swgui"
	"github.com/swaggest/swgui/v5emb"

	"whatsapp-go-api/docs"
)

func registerSwaggerRoutes(app *fiber.App) {
	const swaggerPath = "/swagger"
	const specPath = swaggerPath + "/openapi.yaml"

	app.Get(specPath, func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentType, "application/yaml")
		return c.SendString(docs.OpenAPI)
	})

	ui := v5emb.NewHandlerWithConfig(swgui.Config{
		Title:       "whatsapp-go-api",
		SwaggerJSON: specPath,
		BasePath:    swaggerPath,
		AppendHead: `<meta name="color-scheme" content="dark">
<script>document.documentElement.classList.add("dark-mode");</script>
<style>html.dark-mode body { background: transparent; }</style>`,
	})
	app.Get(swaggerPath, ui)
	app.Get(swaggerPath+"/*", ui)
}
