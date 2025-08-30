# Claude Code Rules

## Output Formatting

### Emoji Usage
- **NEVER** use emojis in any output, CLI messages, or user-facing text
- Only use emojis if the user **explicitly requests** them
- This includes but is not limited to:
  - Status indicators (❌, ✅, ⚠️, etc.)
  - Section headers (📊, 🔄, etc.)
  - Progress indicators
  - Error messages
  - Success messages

### Text Alternatives
Use plain text alternatives instead:
- ✅ → "OK" or "SUCCESS"
- ❌ → "ERROR" 
- ⚠️ → "WARN" or "WARNING"
- 📊 → "SUMMARY" or "ANALYSIS"
- 🔄 → "PROCESSING" or section name

## Examples

**Bad:**
```
✅ No issues found!
❌ ERRORS: 3
📊 ANALYSIS SUMMARY:
```

**Good:**
```
No issues found!
ERRORS: 3
ANALYSIS SUMMARY:
```