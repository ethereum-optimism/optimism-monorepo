import fs from 'fs'

// ---------------------------------------------------------------
// TODO:
// - [x] Support forge invariant tests
// - [x] Support echidna tests
// - [ ] Support multi-line headers within invariant doc comments. (Separate header / desc by blank line)
// ---------------------------------------------------------------

const BASE_INVARIANTS_DIR = `${__dirname}/../contracts/test/invariants`
const BASE_ECHIDNA_DIR = `${__dirname}/../contracts/echidna`
const BASE_DOCS_DIR = `${__dirname}/../invariant-docs`
const BASE_ECHIDNA_GH_URL =
  'https://github.com/ethereum-optimism/optimism/tree/develop/packages/contracts-bedrock/contracts/echidna/'
const BASE_INVARIANT_GH_URL =
  'https://github.com/ethereum-optimism/optimism/tree/develop/packages/contracts-bedrock/contracts/test/invariants/'

// Represents an invariant test contract
type Contract = {
  name: string
  fileName: string
  isEchidna: boolean
  docs: InvariantDoc[]
}

// Represents the documentation of an invariant
type InvariantDoc = {
  header?: string
  desc?: string
  lineNo?: number
}

/**
 * Lazy-parses all test files in the `contracts/test/invariants` directory to generate documentation
 * on all invariant tests.
 */
const docGen = (dir: string): void => {
  // Grab all files within the invariants test dir
  const files = fs.readdirSync(dir)

  // Array to store all found invariant documentation comments.
  const docs: Contract[] = []

  for (const fileName of files) {
    // Read the contents of the invariant test file.
    const fileContents = fs
      .readFileSync(`${dir}/${fileName}`)
      .toString()

    // Split the file into individual lines and trim whitespace.
    const lines = fileContents.split('\n').map((line: string) => line.trim())

    // Create an object to store all invariant test docs for the current contract
    const isEchidna = fileName.startsWith('Fuzz')
    const name = isEchidna ? fileName.replace('Fuzz', '').replace('.sol', '') : fileName.replace('.t.sol', '')
    const contract: Contract = { name, fileName, isEchidna, docs: [] }

    let currentDoc: InvariantDoc

    // Loop through all lines to find comments.
    for (let i = 0; i < lines.length; i++) {
      let line = lines[i]

      if (line.startsWith('/**')) {
        // We are at the beginning of a new doc comment. Reset the `currentDoc`.
        currentDoc = {}

        // Move on to the next line
        line = lines[++i]

        // We have an invariant doc
        if (line.startsWith('* INVARIANT:')) {
          // TODO: Handle ambiguous case for `INVARIANT: ` prefix.
          // Assign the header of the invariant doc.
          currentDoc = {
            header: line.replace('* INVARIANT:', '').trim(),
            desc: '',
          }

          // Process the description
          while ((line = lines[++i]).startsWith('*')) {
            line = line.replace(/\*(\/)?/, '').trim()

            // If the line has any contents, insert it into the desc.
            // Otherwise, consider it a linebreak.
            currentDoc.desc += line.length > 0 ? `${line} ` : '\n'
          }

          // Set the line number of the test
          currentDoc.lineNo = i + 1

          // Add the doc to the contract
          contract.docs.push(currentDoc)
        }
      }
    }

    // Add the contract to the array of docs
    docs.push(contract)
  }

  for (const contract of docs) {
    fs.writeFileSync(
      `${BASE_DOCS_DIR}/${contract.name}.md`,
      renderContractDoc(contract)
    )
  }

  console.log(
    `Generated invariant test documentation for:\n - ${docs.length
    } contracts\n - ${docs.reduce(
      (acc: number, contract: Contract) => acc + contract.docs.length,
      0
    )} invariant tests\nsuccessfully!`
  )
}

/**
 * Render a `Contract` object into valid markdown.
 */
const renderContractDoc = (contract: Contract): string => {
  const header = `# \`${contract.name}\` Invariants`
  const docs = contract.docs
    .map((doc: InvariantDoc) => {
      const line = `L${doc.lineNo}`
      return `## ${doc.header}\n**Test:** [\`${line}\`](${getGithubBase(contract)}${contract.fileName}#${line})\n${doc.desc}`
    })
    .join('\n\n')

  return `${header}\n\n${docs}`
}

/**
  * Get the base URL for the test contract
  */
const getGithubBase = ({ isEchidna }: Contract): string =>
  isEchidna ?
    BASE_ECHIDNA_GH_URL :
    BASE_INVARIANT_GH_URL

// Generate the docs

// Forge
console.log('Generating docs for forge invariants...')
docGen(BASE_INVARIANTS_DIR)

// New line
console.log()

// Echidna
console.log('Generating docs for echidna invariants...')
docGen(BASE_ECHIDNA_DIR)
