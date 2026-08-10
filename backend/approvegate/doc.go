// Package approvegate evaluates Approve Gate eligibility before writing approval metadata.
//
// Phase6.5.6.15.2.1: read-only eligibility only. It does NOT Approve, mutate status,
// Freeze, Execute, alter Strategy selection, or register Cron.
package approvegate
