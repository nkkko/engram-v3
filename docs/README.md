# Running the Documentation Locally

This documentation site is built using [MkDocs](https://www.mkdocs.org/).

## Prerequisites

*   Python and `pip` (Python's package installer) installed.

## Installation

1.  **Install MkDocs:**
    If you don't have MkDocs installed, open your terminal and run:
    ```bash
    pip install mkdocs
    ```
    *(You might also need to install a specific theme if one is used. Check the `mkdocs.yml` configuration file for theme details.)*

## Running the Development Server

1.  **Navigate to the root directory:**
    Make sure your terminal is in the `engram-v3` directory (the one containing the `mkdocs.yml` file and the `docs` folder).

2.  **Start the server:**
    Run the following command:
    ```bash
    mkdocs serve
    ```

3.  **View the site:**
    Open your web browser and go to `http://127.0.0.1:8000`. The site will automatically reload when you save changes to the documentation files or the `mkdocs.yml` configuration.

## Building the Site

To create a static build of the site (usually for deployment), run:

```bash
mkdocs build
```

This will generate the static HTML files in a `site` directory within your project root (`engram-v3/site`).