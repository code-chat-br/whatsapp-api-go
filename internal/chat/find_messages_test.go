package chat

import (
	"testing"

	dbtypes "whatsapp-go-api/internal/database/types"
)

func TestValidateFindMessagesDefaultsPagination(t *testing.T) {
	input := FindMessagesRequest{}
	if err := validateFindMessages(&input); err != nil {
		t.Fatalf("validateFindMessages() error = %v", err)
	}
	if input.Offset != DefaultFindMessagesLimit || input.Page != 1 {
		t.Fatalf("pagination defaults = offset %d page %d", input.Offset, input.Page)
	}
}

func TestFindMessagesFiltersIgnoresEmptyDevice(t *testing.T) {
	device := " "
	input := FindMessagesRequest{Where: FindMessagesWhere{Device: &device}}
	if err := validateFindMessages(&input); err != nil {
		t.Fatalf("validateFindMessages() error = %v", err)
	}
	filters := findMessagesFilters(input.Where)
	if filters.Device != nil {
		t.Fatalf("expected empty device to be ignored, got %q", *filters.Device)
	}
}

func TestFindMessagesFiltersAcceptsValidDevice(t *testing.T) {
	device := string(dbtypes.DeviceMessageWeb)
	input := FindMessagesRequest{Where: FindMessagesWhere{Device: &device}}
	if err := validateFindMessages(&input); err != nil {
		t.Fatalf("validateFindMessages() error = %v", err)
	}
	filters := findMessagesFilters(input.Where)
	if filters.Device == nil || *filters.Device != dbtypes.DeviceMessageWeb {
		t.Fatalf("expected web device filter, got %#v", filters.Device)
	}
}
