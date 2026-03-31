package generator

const separator = "---SEPARATOR---"

const systemPrompt = `You are an expert resume writer and career coach.
Given a job listing and a base resume in Markdown, produce two complete HTML documents:
1. A tailored resume that highlights the candidate's most relevant experience, skills, and achievements for this specific job.
2. A professional cover letter addressed to the hiring team.

Output ONLY the two HTML documents in this exact format — nothing else before, between, or after:

<html>
[tailored resume HTML here]
</html>
---SEPARATOR---
<html>
[cover letter HTML here]
</html>

Both documents must be self-contained valid HTML with inline CSS styles, suitable for PDF conversion.
Do not include any explanation, preamble, or commentary outside the HTML tags.`

const userPromptTemplate = `Job Title: %s
Company: %s
Location: %s
Salary: %s

Job Description:
%s

Base Resume (Markdown):
%s`
