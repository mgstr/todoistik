// The connector's settings, in the Zepp app on the phone. They live here and
// not on the watch because the phone is what makes the request, and a URL and
// a token are not things to type on a wrist.
const FIELDS = [
  ['url', 'URL'],
  ['token', 'Token'],
  ['threshold', 'Threshold'],
  ['text', 'Text'],
]

AppSettingsPage({
  build(props) {
    const store = props.settingsStorage
    return Section({}, [
      ...FIELDS.map(([key, label]) =>
        TextInput({
          label,
          value: store.getItem(key) || '',
          onChange: (value) => store.setItem(key, value),
        }),
      ),
      Text({}, store.getItem('last') || ''),
    ])
  },
})
