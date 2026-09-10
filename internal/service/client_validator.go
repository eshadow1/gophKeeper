package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/eshadow1/gophkeeper/internal/model"
)

const (
	minSizePassword   = 8
	minSizeHolder     = 3
	maxSizeHolder     = 50
	minSizeCardNumber = 13
	maxSizeCardNumber = 20
	minMouth          = 1
	maxMouth          = 12
	validYear2        = 2
	validYear4        = 4
)

// validator инкапсулирует логику валидации различных типов данных (payload
type validator struct {
}

// NewValidator создает и возвращает новый экземпляр валидатора.
func NewValidator() *validator {
	return &validator{}
}

// Validate проверяет корректность предоставленной полезной нагрузки (payload)
// в соответствии с указанным типом элемента (dataType). Метод использует
// утверждение типа (type assertion) для извлечения специфичных структур
// данных и применяет к ним соответствующие бизнес-правила валидации.
func (v *validator) Validate(_ context.Context, dataType model.ItemType, payload any) error {
	switch dataType {
	case model.LoginItem:
		el := payload.(model.LoginPayload)
		if len(el.Password) < minSizePassword {
			return fmt.Errorf("LoginPayload: password is too short")
		}
		if el.Username == "" {
			return fmt.Errorf("LoginPayload: username is empty")
		}
	case model.CardItem:
		card := payload.(model.CardPayload)

		return v.validateCardItem(&card)
	case model.BinaryItem:
		el := payload.(model.BinaryPayload)
		if el.FilePath == "" {
			return fmt.Errorf("BinaryPayload: file path is empty")
		}

		info, err := os.Stat(el.FilePath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("BinaryPayload: file does not exist: %s", el.FilePath)
			}
			return fmt.Errorf("BinaryPayload: %w", err)
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("BinaryPayload: file is not a regular file: %s", el.FilePath)
		}
	case model.TextItem:
		el := payload.(model.TextPayload)
		if el.Content == "" {
			return fmt.Errorf("TextPayload: content is empty")
		}
	default:
		return fmt.Errorf("invalid item type")
	}
	return nil
}

func (*validator) validateCardNumber(number string) error {
	number = strings.ReplaceAll(number, " ", "")
	number = strings.ReplaceAll(number, "-", "")

	if len(number) < minSizeCardNumber || len(number) > maxSizeCardNumber {
		return errors.New("invalid card number")
	}

	for _, ch := range number {
		if ch < '0' || ch > '9' {
			return errors.New("invalid card number")
		}
	}

	return nil
}

func (*validator) validateHolder(holder string) error {
	holder = strings.TrimSpace(holder)
	if len(holder) < minSizeHolder {
		return errors.New("invalid holder")
	}
	if len(holder) > maxSizeHolder {
		return errors.New("invalid holder")
	}

	matched, _ := regexp.MatchString(`^[A-Za-zА-Яа-я\s]+$`, holder)
	if !matched {
		return errors.New("invalid holder")
	}

	return nil
}

func (*validator) validateMonth(month string) error {
	if len(month) != 2 {
		return errors.New("invalid month")
	}

	m, err := strconv.Atoi(month)
	if err != nil {
		return errors.New("invalid month")
	}

	if m < minMouth || m > maxMouth {
		return errors.New("invalid month")
	}

	return nil
}

func (*validator) validateYear(year string) error {
	if len(year) != validYear2 && len(year) != validYear4 {
		return errors.New("invalid year")
	}

	_, err := strconv.Atoi(year)
	if err != nil {
		return errors.New("invalid year")
	}

	return nil
}

func (*validator) validateCVV(cvv string) error {
	matched, _ := regexp.MatchString(`^\d{3,4}$`, cvv)
	if !matched {
		return errors.New("invalid CVV")
	}
	return nil
}

func (v *validator) validateCardItem(card *model.CardPayload) error {
	if err := v.validateCardNumber(card.Number); err != nil {
		return fmt.Errorf("номер карты: %w", err)
	}

	if err := v.validateHolder(card.Holder); err != nil {
		return fmt.Errorf("имя держателя: %w", err)
	}

	if err := v.validateMonth(card.ExpiryMonth); err != nil {
		return fmt.Errorf("месяц: %w", err)
	}

	if err := v.validateYear(card.ExpiryYear); err != nil {
		return fmt.Errorf("год: %w", err)
	}

	if err := v.validateCVV(card.CVV); err != nil {
		return fmt.Errorf("CVV: %w", err)
	}

	return nil
}
