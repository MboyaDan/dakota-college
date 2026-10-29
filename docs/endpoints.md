# API endpoints

Generated from the route table by `TestEndpointInventoryIsCurrent` (backend/internal/httpapi/hardening_test.go). Do not edit by hand: run
`UPDATE_ENDPOINTS=1 go test ./internal/httpapi -run TestEndpointInventoryIsCurrent` after changing a route.

Every route needs a session unless marked public. Staff routes also need the named permission and a staff account; applicant and student routes act only on the caller's own records. Changes (POST, PUT, DELETE) on signed-in routes also need the CSRF token. The review is in [security.md](security.md).

| Method | Path | Who may call it |
|---|---|---|
| GET | `/api/v1/health` | Public |
| GET | `/api/v1/ready` | Public |
| POST | `/api/v1/auth/login` | Public |
| POST | `/api/v1/auth/password-reset` | Public |
| POST | `/api/v1/auth/password-reset/confirm` | Public |
| GET | `/api/v1/auth/session` | Any signed-in account |
| POST | `/api/v1/auth/logout` | Any signed-in account |
| POST | `/api/v1/auth/login/code` | Public |
| GET | `/api/v1/auth/two-step` | Any staff account |
| POST | `/api/v1/auth/two-step/setup` | Any staff account |
| POST | `/api/v1/auth/two-step/confirm` | Any staff account |
| POST | `/api/v1/auth/two-step/recovery-codes` | Any staff account |
| POST | `/api/v1/auth/two-step/disable` | Any staff account |
| GET | `/api/v1/admin/users` | Staff with `users.manage` |
| POST | `/api/v1/admin/users` | Staff with `users.manage` |
| PUT | `/api/v1/admin/users/:id/roles` | Staff with `users.manage` |
| POST | `/api/v1/admin/users/:id/disable` | Staff with `users.manage` |
| POST | `/api/v1/admin/users/:id/enable` | Staff with `users.manage` |
| POST | `/api/v1/admin/users/:id/two-step/reset` | Staff with `users.manage` |
| GET | `/api/v1/admin/roles` | Staff with `roles.manage` |
| POST | `/api/v1/admin/roles` | Staff with `roles.manage` |
| PUT | `/api/v1/admin/roles/:id/permissions` | Staff with `roles.manage` |
| DELETE | `/api/v1/admin/roles/:id` | Staff with `roles.manage` |
| GET | `/api/v1/admin/permissions` | Staff with `roles.manage` |
| GET | `/api/v1/staff/content/:kind` | Staff with `content.read` |
| POST | `/api/v1/staff/content/:kind` | Staff with `content.write` |
| GET | `/api/v1/staff/content/:kind/:id` | Staff with `content.read` |
| PUT | `/api/v1/staff/content/:kind/:id` | Staff with `content.write` |
| DELETE | `/api/v1/staff/content/:kind/:id` | Staff with `content.write` |
| DELETE | `/api/v1/staff/content/:kind/:id/pending` | Staff with `content.write` |
| POST | `/api/v1/staff/content/:kind/:id/publish` | Staff with `content.publish` |
| POST | `/api/v1/staff/content/:kind/:id/unpublish` | Staff with `content.publish` |
| POST | `/api/v1/staff/content/:kind/:id/archive` | Staff with `content.publish` |
| GET | `/api/v1/staff/settings` | Staff with `content.read` |
| PUT | `/api/v1/staff/settings` | Staff with `content.publish` |
| POST | `/api/v1/staff/images` | Staff with `content.write` |
| GET | `/api/v1/staff/images/:id` | Staff with `content.read` |
| GET | `/api/v1/staff/enquiries` | Staff with `enquiries.view` |
| GET | `/api/v1/staff/enquiries/counts` | Staff with `enquiries.view` |
| GET | `/api/v1/staff/enquiries/assignees` | Staff with `enquiries.manage` |
| GET | `/api/v1/staff/enquiries/:id` | Staff with `enquiries.view` |
| POST | `/api/v1/staff/enquiries/:id/assign` | Staff with `enquiries.manage` |
| POST | `/api/v1/staff/enquiries/:id/status` | Staff with `enquiries.manage` |
| POST | `/api/v1/staff/enquiries/:id/notes` | Staff with `enquiries.manage` |
| GET | `/api/v1/staff/notification-templates` | Staff with `notifications.manage` |
| PUT | `/api/v1/staff/notification-templates/:key/:channel` | Staff with `notifications.manage` |
| GET | `/api/v1/staff/notifications` | Staff with `notifications.manage` |
| GET | `/api/v1/staff/notifications/counts` | Staff with `notifications.manage` |
| POST | `/api/v1/staff/notifications/:id/resend` | Staff with `notifications.manage` |
| POST | `/api/v1/applicant/sign-in-link` | Public |
| POST | `/api/v1/applicant/resume` | Public |
| POST | `/api/v1/applicant/sign-in` | Public |
| GET | `/api/v1/applicant/applications` | The applicant, own records only |
| POST | `/api/v1/applicant/applications` | The applicant, own records only |
| GET | `/api/v1/applicant/applications/:id` | The applicant, own records only |
| PUT | `/api/v1/applicant/applications/:id/intake` | The applicant, own records only |
| PUT | `/api/v1/applicant/applications/:id/personal` | The applicant, own records only |
| PUT | `/api/v1/applicant/applications/:id/guardian` | The applicant, own records only |
| PUT | `/api/v1/applicant/applications/:id/education` | The applicant, own records only |
| PUT | `/api/v1/applicant/applications/:id/sponsor` | The applicant, own records only |
| POST | `/api/v1/applicant/applications/:id/documents` | The applicant, own records only |
| DELETE | `/api/v1/applicant/applications/:id/documents/:doc` | The applicant, own records only |
| POST | `/api/v1/applicant/applications/:id/documents/:doc/link` | The applicant, own records only |
| POST | `/api/v1/applicant/applications/:id/submit` | The applicant, own records only |
| POST | `/api/v1/applicant/applications/:id/reply` | The applicant, own records only |
| POST | `/api/v1/applicant/applications/:id/accept-offer` | The applicant, own records only |
| POST | `/api/v1/applicant/applications/:id/withdraw` | The applicant, own records only |
| POST | `/api/v1/applicant/applications/:id/offer-letter/link` | The applicant, own records only |
| POST | `/api/v1/staff/applications/:id/documents/:doc/link` | Staff with `admissions.view` |
| GET | `/api/v1/files/:token` | Any signed-in account |
| GET | `/api/v1/staff/reports/dashboard` | Staff with `reports.view` |
| GET | `/api/v1/staff/reports/exports/:kind` | Staff with `reports.export` |
| GET | `/api/v1/staff/applications` | Staff with `admissions.view` |
| GET | `/api/v1/staff/applications/counts` | Staff with `admissions.view` |
| GET | `/api/v1/staff/applications/assignees` | Staff with `admissions.review` |
| GET | `/api/v1/staff/applications/report` | Staff with `admissions.view` |
| GET | `/api/v1/staff/applications/:id` | Staff with `admissions.view` |
| POST | `/api/v1/staff/applications/:id/assign` | Staff with `admissions.review` |
| POST | `/api/v1/staff/applications/:id/notes` | Staff with `admissions.review` |
| POST | `/api/v1/staff/applications/:id/request-info` | Staff with `admissions.review` |
| POST | `/api/v1/staff/applications/:id/resume-review` | Staff with `admissions.review` |
| POST | `/api/v1/staff/applications/:id/reject` | Staff with `admissions.offer` |
| POST | `/api/v1/staff/applications/:id/offer` | Staff with `admissions.offer` |
| POST | `/api/v1/staff/applications/:id/offer-letter/link` | Staff with `admissions.view` |
| POST | `/api/v1/staff/applications/:id/enrol` | Staff with `admissions.enrol` |
| GET | `/api/v1/staff/students` | Staff with `students.view` |
| GET | `/api/v1/staff/students/:id` | Staff with `students.view` |
| POST | `/api/v1/staff/students/:id/status` | Staff with `students.update` |
| POST | `/api/v1/staff/students/:id/cohort` | Staff with `students.update` |
| POST | `/api/v1/staff/students/:id/guardians` | Staff with `students.update` |
| PUT | `/api/v1/staff/students/:id/guardians/:item` | Staff with `students.update` |
| DELETE | `/api/v1/staff/students/:id/guardians/:item` | Staff with `students.update` |
| GET | `/api/v1/staff/cohorts` | Staff with `students.view` |
| POST | `/api/v1/staff/cohorts` | Staff with `academics.manage` |
| GET | `/api/v1/staff/terms` | Any staff account |
| GET | `/api/v1/staff/units` | Any staff account |
| GET | `/api/v1/staff/units/:id` | Any staff account |
| GET | `/api/v1/staff/programmes/:id/units` | Any staff account |
| POST | `/api/v1/staff/terms` | Staff with `academics.manage` |
| PUT | `/api/v1/staff/terms/:id` | Staff with `academics.manage` |
| POST | `/api/v1/staff/units` | Staff with `academics.manage` |
| PUT | `/api/v1/staff/units/:id` | Staff with `academics.manage` |
| PUT | `/api/v1/staff/programmes/:id/units` | Staff with `academics.manage` |
| GET | `/api/v1/staff/trainers` | Staff with `academics.manage` |
| GET | `/api/v1/staff/offerings` | Staff with `academics.manage` |
| POST | `/api/v1/staff/offerings` | Staff with `academics.manage` |
| GET | `/api/v1/staff/offerings/:id` | Staff with `academics.manage` |
| PUT | `/api/v1/staff/offerings/:id/trainers` | Staff with `academics.manage` |
| POST | `/api/v1/staff/offerings/:id/timetable` | Staff with `academics.manage` |
| DELETE | `/api/v1/staff/offerings/:id/timetable/:item` | Staff with `academics.manage` |
| POST | `/api/v1/staff/offerings/:id/registrations` | Staff with `academics.manage` |
| POST | `/api/v1/staff/offerings/:id/registrations/:item/drop` | Staff with `academics.manage` |
| GET | `/api/v1/staff/teaching` | Staff with `academics.teach` |
| GET | `/api/v1/staff/teaching/:id` | Staff with `academics.teach` |
| GET | `/api/v1/admin/users/:id/schools` | Staff with `users.manage` |
| PUT | `/api/v1/admin/users/:id/schools` | Staff with `users.manage` |
| POST | `/api/v1/student/sign-in` | Public |
| POST | `/api/v1/student/password-reset` | Public |
| GET | `/api/v1/student/dashboard` | The student, own records only |
| GET | `/api/v1/student/profile` | The student, own records only |
| PUT | `/api/v1/student/profile/phone` | The student, own records only |
| POST | `/api/v1/student/profile/change-request` | The student, own records only |
| POST | `/api/v1/student/guardians` | The student, own records only |
| PUT | `/api/v1/student/guardians/:item` | The student, own records only |
| DELETE | `/api/v1/student/guardians/:item` | The student, own records only |
| GET | `/api/v1/student/registration` | The student, own records only |
| POST | `/api/v1/student/registration/:id` | The student, own records only |
| POST | `/api/v1/student/registration/:id/drop` | The student, own records only |
| GET | `/api/v1/student/timetable` | The student, own records only |
| GET | `/api/v1/student/notices` | The student, own records only |
| GET | `/api/v1/student/requests` | The student, own records only |
| POST | `/api/v1/student/requests` | The student, own records only |
| GET | `/api/v1/staff/notices` | Staff with `academics.manage` |
| POST | `/api/v1/staff/notices` | Staff with `academics.manage` |
| POST | `/api/v1/staff/notices/:id/withdraw` | Staff with `academics.manage` |
| GET | `/api/v1/staff/teaching/:id/notices` | Staff with `academics.teach` |
| POST | `/api/v1/staff/teaching/:id/notices` | Staff with `academics.teach` |
| POST | `/api/v1/staff/teaching/:id/notices/:item/withdraw` | Staff with `academics.teach` |
| GET | `/api/v1/staff/service-requests` | Staff with `students.view` |
| POST | `/api/v1/staff/service-requests/:id` | Staff with `students.update` |
| GET | `/api/v1/staff/profile-requests` | Staff with `students.view` |
| POST | `/api/v1/staff/profile-requests/:id/approve` | Staff with `students.update` |
| POST | `/api/v1/staff/profile-requests/:id/decline` | Staff with `students.update` |
| GET | `/api/v1/staff/programmes/:id/assessment` | Any staff account |
| PUT | `/api/v1/staff/programmes/:id/assessment` | Staff with `academics.manage` |
| GET | `/api/v1/staff/teaching/:id/results` | Staff with `academics.marks.enter` |
| PUT | `/api/v1/staff/teaching/:id/results/marks` | Staff with `academics.marks.enter` |
| POST | `/api/v1/staff/teaching/:id/results/submit` | Staff with `academics.marks.enter` |
| GET | `/api/v1/staff/result-approvals` | Staff with `academics.results.approve` |
| GET | `/api/v1/staff/result-approvals/:id` | Staff with `academics.results.approve` |
| POST | `/api/v1/staff/result-approvals/:id/approve` | Staff with `academics.results.approve` |
| POST | `/api/v1/staff/result-approvals/:id/return` | Staff with `academics.results.approve` |
| GET | `/api/v1/staff/result-releases` | Staff with `academics.results.release` |
| GET | `/api/v1/staff/result-releases/:id` | Staff with `academics.results.release` |
| POST | `/api/v1/staff/result-releases/:id/release` | Staff with `academics.results.release` |
| GET | `/api/v1/staff/students/:id/results` | Staff with `students.view` |
| POST | `/api/v1/staff/students/:id/transcripts` | Staff with `academics.results.release` |
| POST | `/api/v1/staff/students/:id/transcripts/:item/link` | Staff with `students.view` |
| GET | `/api/v1/student/results` | The student, own records only |
| GET | `/api/v1/student/results/slip` | The student, own records only |
| POST | `/api/v1/student/transcripts/:id/link` | The student, own records only |
| GET | `/api/v1/staff/finance/summary` | Staff with `finance.view` |
| GET | `/api/v1/staff/finance/programmes` | Staff with `finance.view` |
| GET | `/api/v1/staff/finance/fee-structures` | Staff with `finance.view` |
| POST | `/api/v1/staff/finance/fee-structures` | Staff with `finance.invoice` |
| GET | `/api/v1/staff/finance/fee-structures/:id` | Staff with `finance.view` |
| PUT | `/api/v1/staff/finance/fee-structures/:id` | Staff with `finance.invoice` |
| DELETE | `/api/v1/staff/finance/fee-structures/:id` | Staff with `finance.invoice` |
| GET | `/api/v1/staff/finance/invoices` | Staff with `finance.view` |
| POST | `/api/v1/staff/finance/invoices/issue` | Staff with `finance.invoice` |
| GET | `/api/v1/staff/finance/invoices/:id` | Staff with `finance.view` |
| PUT | `/api/v1/staff/finance/invoices/:id` | Staff with `finance.invoice` |
| POST | `/api/v1/staff/finance/invoices/:id/fill` | Staff with `finance.invoice` |
| POST | `/api/v1/staff/finance/invoices/:id/issue` | Staff with `finance.invoice` |
| POST | `/api/v1/staff/finance/invoices/:id/void` | Staff with `finance.invoice` |
| GET | `/api/v1/staff/finance/invoices/:id/pdf` | Staff with `finance.view` |
| POST | `/api/v1/staff/finance/billing` | Staff with `finance.invoice` |
| GET | `/api/v1/staff/finance/payments` | Staff with `finance.view` |
| POST | `/api/v1/staff/finance/payments` | Staff with `finance.payment.record` |
| GET | `/api/v1/staff/finance/payments/:id` | Staff with `finance.view` |
| POST | `/api/v1/staff/finance/payments/:id/confirm` | Staff with `finance.payment.confirm` |
| POST | `/api/v1/staff/finance/payments/:id/reject` | Staff with `finance.payment.confirm` |
| GET | `/api/v1/staff/finance/payments/:id/receipt` | Staff with `finance.view` |
| GET | `/api/v1/staff/finance/arrears` | Staff with `finance.view` |
| GET | `/api/v1/staff/finance/students/:id` | Staff with `finance.view` |
| GET | `/api/v1/staff/finance/students/:id/statement` | Staff with `finance.view` |
| POST | `/api/v1/staff/finance/students/:id/invoices` | Staff with `finance.invoice` |
| POST | `/api/v1/staff/finance/students/:id/adjustments` | Staff with `finance.adjust` |
| GET | `/api/v1/student/fees` | The student, own records only |
| GET | `/api/v1/student/fees/statement` | The student, own records only |
| GET | `/api/v1/student/fees/invoices/:id` | The student, own records only |
| GET | `/api/v1/student/fees/payments/:id/receipt` | The student, own records only |
| POST | `/api/v1/payments/c2b/:secret/validation` | Public, on a secret path (payment provider) |
| POST | `/api/v1/payments/c2b/:secret/confirmation` | Public, on a secret path (payment provider) |
| GET | `/api/v1/staff/finance/mpesa` | Staff with `finance.view` |
| GET | `/api/v1/staff/finance/mpesa/transactions` | Staff with `finance.view` |
| POST | `/api/v1/staff/finance/mpesa/transactions/:id/assign` | Staff with `finance.payment.confirm` |
| POST | `/api/v1/staff/finance/mpesa/transactions/:id/refund` | Staff with `finance.payment.confirm` |
| GET | `/api/v1/staff/finance/mpesa/reconciliation` | Staff with `finance.view` |
| POST | `/api/v1/staff/finance/mpesa/register-urls` | Staff with `finance.mpesa.setup` |
| POST | `/api/v1/staff/finance/mpesa/simulate` | Staff with `finance.mpesa.setup` |
| POST | `/api/v1/staff/finance/mpesa/reprocess` | Staff with `finance.mpesa.setup` |
| POST | `/api/v1/payments/kcb/:secret/notification` | Public, on a secret path (payment provider) |
| GET | `/api/v1/staff/finance/kcb` | Staff with `finance.view` |
| GET | `/api/v1/staff/finance/kcb/transactions` | Staff with `finance.view` |
| POST | `/api/v1/staff/finance/kcb/transactions/:id/assign` | Staff with `finance.payment.confirm` |
| POST | `/api/v1/staff/finance/kcb/transactions/:id/refund` | Staff with `finance.payment.confirm` |
| GET | `/api/v1/staff/finance/kcb/reconciliation` | Staff with `finance.view` |
| POST | `/api/v1/staff/finance/kcb/reprocess` | Staff with `finance.mpesa.setup` |
| GET | `/api/v1/public/site` | Public |
| POST | `/api/v1/public/enquiries` | Public |
| GET | `/api/v1/public/news` | Public |
| GET | `/api/v1/public/news/:slug` | Public |
| GET | `/api/v1/public/events` | Public |
| GET | `/api/v1/public/events/:slug` | Public |

215 routes: 20 public, 155 staff, 16 applicant, 21 student, 3 for any signed-in account.
