import { OfflineBanner } from './network/OfflineBanner'
import AnimalPokeApp from './features/animal-poke/AnimalPokeApp'
import { AppProviders } from './providers/AppProviders'

const App: React.FC = () => {
  return (
    <AppProviders>
      <><OfflineBanner /><AnimalPokeApp /></>
    </AppProviders>
  )
}

export default App
