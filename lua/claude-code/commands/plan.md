---
description: Create BDD-like specifications for features through guided planning
allowed-tools:
  - write
  - read
  - bash
---

# Plan - BDD Specification Builder

This command helps you create behavior-driven development (BDD) style specifications by guiding you through a structured planning process. The output will be a feature specification document saved in the `.ai/` directory.

## Process Overview

I'll help you build a comprehensive feature specification by asking targeted questions about:

1. **Feature Overview**
   - Feature name and description
   - Business value and goals
   - Target users/personas

2. **User Stories**
   - As a [user type]
   - I want [functionality]
   - So that [benefit]

3. **Acceptance Criteria**
   - Given [context/precondition]
   - When [action/event]
   - Then [expected outcome]

4. **Technical Considerations**
   - Dependencies and constraints
   - Non-functional requirements
   - Edge cases and error scenarios

5. **Success Metrics**
   - How to measure success
   - Key performance indicators

## Let's Begin

I'll guide you through creating a BDD-style specification. First, let me create the `.ai` directory if it doesn't exist:

```bash
mkdir -p .ai
```

Now, let's start with the basics:

### 1. Feature Overview

**What is the name of the feature you want to plan?**
Please provide a short, descriptive name (e.g., "User Authentication", "Shopping Cart", "Email Notifications")

After you provide the feature name, I'll continue asking questions to build a complete specification. Each answer you provide will help create a more detailed and actionable feature description.

The final output will be saved as `.ai/[feature-name]-spec.md` with a structured format that includes:
- Feature description
- User stories with acceptance criteria
- Technical requirements
- Testing scenarios
- Implementation notes

Please start by telling me the feature name, and I'll guide you through the rest of the planning process.