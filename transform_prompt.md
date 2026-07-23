Create a new project of this existing project as a Rust project in a subdirectory. 
* Don't make the target run in an IDE if you are connected to an IDE, only install the most basic development tools, e.g. a compiler.s
* Make the project compile and run.

Todos:
1. Understand and analyze the existing project.
2. Plan the migration with in a file called migration_plan.md.
3. Use the installed cli tool 'rinku' to find replacedments for the libraries and dependencies used.
  - if no replacement is found use the --unsafe flag and document the unsafe dependency in the migration_plan.md file.
4. If there is a CLI api, document the CLI and make sure the new is the same as the existing one.
   - if there is CLI output, make sure the output is the same.
5. If there is a Web app, document the URL structure and make sure the new is the same.
   - do not change HTML or CSS or other assets, directly translate one templating language to the new one.
6. If there is a REST API, document the API and make sure the new is the same.
7. Use standard library functionality as much as possible.
8. Drop functionality if there is no replacement library in that language - don't use C if not told explicitly.
9. Document functionality that is dropped at the top of the migration_plan.md file.

If there are tests, independently migrate the tests to the new project as closely as possible with the
target testing solution.

After creating the plan, check the plan to make sure your confident that it will work.

When asked to implement the plan, make sure the projects compiles and there are no errors
or warnings. Fix warnings and errors.

Failing tests show that there were mistakes with migration. Decide if the 
tests or the code needs to be fixed according to the plan.
