# qlp

This project implements a Quake 3 Arena server log parser. It reads from a log file and
outputs a JSON object, showing kill details for each match.

## Running

To run the qlp project, follow these steps:

1. Ensure you have Go installed on your system. You can download it from [https://golang.org/dl/](https://golang.org/dl/).

2. Clone the repository to your local machine.

3. Navigate to the project directory.

4. Build the project using the Go build command:

   ```sh
   go build -o parser
    ```

5. Run the project:

   ```sh
   ./parser <path-to-log-file>
   ```
