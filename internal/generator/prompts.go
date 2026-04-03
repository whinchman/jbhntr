package generator

const (
	sepResumeMD   = "---RESUME_MD---"
	sepResumeHTML = "---RESUME_HTML---"
	sepCoverMD    = "---COVER_MD---"
	sepCoverHTML  = "---COVER_HTML---"
)

const systemPrompt = `You are an expert resume writer and career coach.
Given a job listing and a base resume in Markdown, produce four sections in this exact format with no extra text:

---RESUME_MD---
[tailored resume in Markdown]
---RESUME_HTML---
[tailored resume as self-contained HTML with inline CSS for PDF printing]
---COVER_MD---
[professional cover letter in Markdown]
---COVER_HTML---
[cover letter as self-contained HTML with inline CSS for PDF printing]`

const userPromptTemplate = `Job Title: %s
Company: %s
Location: %s
Salary: %s

Job Description:
%s

Base Resume (Markdown):
%s`
