package request

import (
	"context"
	"time"

	"github.com/turnerlabs/udeploy/component/auth"

	"github.com/turnerlabs/udeploy/component/user"

	"github.com/labstack/echo/v4"
	"github.com/turnerlabs/udeploy/component/db"
	"github.com/turnerlabs/udeploy/model"
	"go.mongodb.org/mongo-driver/mongo"
)

const timeoutSeconds = 120

// Context ...
func Context(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctxParent, cancel := context.WithTimeout(context.Background(), timeoutSeconds*time.Second)
		defer cancel()

		session, err := db.Client().StartSession()
		if err != nil {
			return err
		}

		var ctx context.Context

		if err = mongo.WithSession(ctxParent, session, func(sctx mongo.SessionContext) error {

			user, err := user.Get(sctx, c.Get(auth.UserIDParam).(string))
			if err != nil {
				return err
			}

			ctx = context.WithValue(ctxParent, model.ContextKey("user"), user)

			return nil
		}); err != nil {
			return err
		}

		err = mongo.WithSession(ctx, session, func(sctx mongo.SessionContext) error {
			c.Set("ctx", sctx)

			return next(c)
		})

		session.EndSession(ctx)

		return err
	}
}
