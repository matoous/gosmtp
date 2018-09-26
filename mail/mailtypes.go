package mail

type MailType string

const (
	MailTypeImportant    MailType = "Important"    // important emails
	MailTypeDiscount              = "Discount"     // discount offers
	MailTypeTicket                = "Ticket"       // plain/train/event tickets
	MailTypeReceipt               = "Receipt"      // shop receipts
	MailTypeDelivery              = "Delivery"     // package deliveries etc.
	MailTypeQuestion              = "Question"     //
	MailTypeEducation             = "Education"    //
	MailTypeInvitation            = "Invitation"   // event invitations etc.
	MailTypeCelebration           = "Celebration"  // birthday congratulations etc.
	MailTypeSecurity              = "Security"     // security email, e.g. someone logged to your account from china
	MailTypeConfirmation          = "Confirmation" // site registration etc.
)
