package model

import (
	"github.com/go-pg/pg"
	"github.com/integration-system/isp-lib/database"
	"github.com/integration-system/isp-lib/structure"
)

func GetToken(tokenString string) (*structure.AdminToken, error) {
	token := structure.AdminToken{}
	err := database.GetDBManager().Db.Model(&token).
		Where("token = ?", tokenString).
		Where("now() < expired_at").
		Select()
	if err != nil && err == pg.ErrNoRows {
		return nil, nil
	}
	return &token, err
}
