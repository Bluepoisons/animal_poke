import AnimalPokeApp from './features/animal-poke/AnimalPokeApp'
import { AppProviders } from './providers/AppProviders'

const App: React.FC = () => {
  return (
    <AppProviders>
      <AnimalPokeApp />
    </AppProviders>
  )
}

export default App
