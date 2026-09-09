package pipeline

import (
	"papertrail/internal/document"
)

func shouldStoreEntity(entityType document.EntityType) bool {
	switch entityType {
	case
		document.EntityTypePerson,
		document.EntityTypeOrganisation,
		document.EntityTypeProduct,
		document.EntityTypeIDNumber,
		document.EntityTypeEvent,
		document.EntityTypeAddress,
		document.EntityTypeVehicle,
		document.EntityTypeLocation,
		document.EntityTypeMoney,
		document.EntityTypeDate,
		document.EntityTypeEmail,
		document.EntityTypePhoneNumber,
		document.EntityTypeJobTitle:
		return true
	default:
		return false
	}
}
