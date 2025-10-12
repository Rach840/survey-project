package providers

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SurveyProvider struct {
	db *pgxpool.Pool
}

func NewSurveyProvider(pg *pgxpool.Pool) *SurveyProvider {
	return &SurveyProvider{
		db: pg,
	}
}

func (s SurveyProvider) CreateMagicToken(ctx context.Context, token string, userID uint, expiresAt time.Time) error {
	//tx, err := s.db.Begin(ctx)
	//if err != nil {
	//	return fmt.Errorf("Ошибка начала транзакции")
	//}
	//defer tx.Rollback(ctx)
	//_, err = tx.Exec(ctx,
	//	`INSERT INTO magic_tokens (token, user_id, expires_at, used)
	//     VALUES ($1, $2, $3, FALSE)`,
	//	token,
	//	userID,
	//	expiresAt,
	//)
	//return err
	return nil
}

func (s SurveyProvider) FetchMagicToken(ctx context.Context, token string) error {
	//tx, err := s.db.Begin(ctx)
	//if err != nil {
	//	return fmt.Errorf("Ошибка начала транзакции"), nil
	//}
	//defer tx.Rollback(ctx)
	//var magicToken domain.MagicToken
	//err = tx.QueryRow(ctx,
	//	`SELECT id, token, user_id, expires_at, used, created_at, updated_at
	//     FROM magic_tokens
	//     WHERE token = $1 AND used = FALSE`,
	//	token,
	//).Scan(
	//	&magicToken.ID,
	//	&magicToken.Token,
	//	&magicToken.UserID,
	//	&magicToken.ExpiresAt,
	//	&magicToken.Used,
	//	&magicToken.CreatedAt,
	//	&magicToken.UpdatedAt,
	//)
	//if err != nil {
	//	return domain.MagicToken{}, err
	//}
	//return magicToken, nil
	return nil
}

func (s SurveyProvider) MarkMagicTokenUsed(ctx context.Context, id uint) error {
	//tx, err := s.db.Begin(ctx)
	//if err != nil {
	//	return fmt.Errorf("Ошибка начала транзакции")
	//}
	//defer tx.Rollback(ctx)
	//_, err = tx.Exec(ctx, `UPDATE magic_tokens SET used = TRUE, updated_at = NOW() WHERE id = $1`, id)
	//return err
	return nil
}
