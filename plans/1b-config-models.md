# Plan: 1B — Config & Models

## Overview
Config loading is already done in 1A. This feature adds the data models in
internal/models/ and adds model validation tests. The backlog item also
mentions re-testing config parsing — those tests already exist and pass.

## Steps

### Step 1: Implement internal/models/models.go
- JobStatus string type with constants: Discovered, Notified, Approved, Rejected,
  Generating, Complete, Failed
- JobStatus.Valid() method
- Job struct: all DB fields (ID, ExternalID, Source, Title, Company, Location,
  Description, Salary, ApplyURL, Status, ResumeHTML, CoverHTML, ResumePDF,
  CoverPDF, ErrorMsg, DiscoveredAt, UpdatedAt)
- SearchFilter struct: Keywords, Location, MinSalary, MaxSalary, Title

### Step 2: Write tests in internal/models/models_test.go
- Table-driven tests for JobStatus.Valid() (valid statuses, invalid string)
- Test that all status constants are valid
- Test that an unknown string is not valid

## Files Created/Modified
- internal/models/models.go  (replaces stub doc.go)
- internal/models/models_test.go
